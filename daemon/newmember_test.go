package daemon

import (
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A new member, from the invite on, through the hosts' own requests to the
// cloud as the apps make them: they see the team's notes and projects,
// open a session on the team's host as their own agent account, and see
// nobody's credentials and nobody's personal space (docs/cloud.md).
func TestNewMemberGetsTheTeamAndNothingMore(t *testing.T) {
	t.Setenv(proto.EnvTeamListen, "127.0.0.1:0")
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o700)
	os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte("ada's own sign-in"), 0o600)
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"home=$CLAUDE_CONFIG_DIR\"\nsleep 30\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")

	// Ada makes the team, its note and project, keeps a diary of her own,
	// and shares her host's copy of the project.
	var team api.Team
	name := "Design"
	if err := ada.c.CloudCall("POST", api.PathTeams, api.TeamParams{Name: &name}, &team); err != nil {
		t.Fatal(err)
	}
	var teams []api.Team
	ada.c.CloudCall("GET", api.PathTeams, nil, &teams) // her host learns its teams, as the app does
	ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Roadmap", Team: team.ID}, Body: "the team's plan"})
	ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Diary"}, Body: "ada's own"})
	if _, err := ada.c.AddTeamProject(proto.TeamProjectAddParams{Team: team.ID, Repo: "https://github.com/enowx/app"}); err != nil {
		t.Fatal(err)
	}
	project := gitRepo(t)
	exec.Command("git", "-C", project, "remote", "add", "origin", "git@github.com:enowx/app.git").Run()
	ada.c.AddProject(project)
	private, _ := ada.c.CreateSession(proto.SessionCreateParams{Name: "ada's own", Dir: project, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})
	ada.c.SetSessionPrivate(private.ID, true)
	allow := true
	sh, err := ada.c.SetHostShareTeam(proto.HostShareTeam{Team: team.ID, Projects: []string{project}, AllowSessions: &allow})
	if err != nil || sh.Addr == "" {
		t.Fatalf("shared: %+v %v", sh, err)
	}

	// Bob is invited, and joins from his host.
	var inv api.Invite
	if err := ada.c.CloudCall("POST", strings.Replace(api.PathInvites, "{team}", team.ID, 1), api.InviteParams{Role: api.RoleMember}, &inv); err != nil {
		t.Fatal(err)
	}
	if err := bob.c.CloudCall("POST", api.PathJoin, api.JoinParams{Token: inv.URL}, nil); err != nil {
		t.Fatal(err)
	}
	bob.c.CloudCall("GET", api.PathTeams, nil, &teams)
	ada.sync(t, team.ID)
	bob.sync(t, team.ID) // waits for the key: ada's host seals it for him next
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)

	// The team's notes and projects, not ada's own.
	notes, _ := bob.c.Notes()
	titles := map[string]bool{}
	for _, n := range notes {
		titles[n.Title] = true
	}
	if !titles["Roadmap"] || titles["Diary"] {
		t.Fatalf("bob's notes: %+v", titles)
	}
	if n, ok := bob.noteTitled(t, "Roadmap"); !ok || n.Body != "the team's plan" {
		t.Fatalf("the team's note: %+v", n)
	}
	if list, _ := bob.c.TeamProjects(); len(list) != 1 || list[0].Name != "app" {
		t.Fatalf("bob's team projects: %+v", list)
	}
	hosts, _ := bob.c.TeamHosts()
	if len(hosts) != 1 {
		t.Fatalf("the team's hosts, on bob's: %+v", hosts)
	}

	// On ada's host, with a ticket his host asked the cloud for.
	var tk api.HostTicket
	if err := bob.c.CloudCall("POST", strings.Replace(api.PathHostTickets, "{team}", team.ID, 1), api.HostTicketParams{Host: hosts[0].Device}, &tk); err != nil {
		t.Fatal(err)
	}
	nc, err := net.Dial("tcp", hosts[0].Addr)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := client.New(nc, nil)
	t.Cleanup(func() { v.Close() })
	if _, err := v.TeamAuth(tk.Ticket); err != nil {
		t.Fatal(err)
	}
	if list, _ := v.Sessions(); len(list) != 0 {
		t.Fatalf("ada's private session showed: %+v", list)
	}
	info, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: project, Cols: 100, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := ada.srv.sessions.Get(info.ID)
	accHome := ada.srv.accounts.Home(info.Account)
	waitFor(t, "bob's own account", func() bool { return strings.Contains(sess.Terminal().Text(), "home="+accHome) })
	if data, _ := os.ReadFile(filepath.Join(accHome, ".credentials.json")); len(data) > 0 {
		t.Fatal("bob's session has ada's sign-in")
	}

	// Nobody's credentials, nobody's own space.
	if _, err := v.VaultIndex(); err == nil {
		t.Fatal("a member read the host's vault index")
	}
	if _, err := v.VaultGet(); err == nil {
		t.Fatal("a member read the host's vault")
	}
	if _, err := v.VaultAsk(proto.VaultAsk{Action: proto.VaultSession, Session: info.ID}); err == nil {
		t.Fatal("a member asked the host's vault for credentials")
	}
	if _, err := v.CloudStatus(); err == nil {
		t.Fatal("a member read the host's cloud sign-in")
	}
	if _, err := v.Notes(); err == nil {
		t.Fatal("a member read ada's notes on her host")
	}
	if _, err := v.PeekSession(proto.SessionPeekParams{Session: private.ID}); err == nil {
		t.Fatal("a member read ada's private session")
	}
}
