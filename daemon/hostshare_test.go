package daemon

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A host shared with a team: its user picks projects, the team listener
// opens, the host is announced in the team's space and listed on the
// members' hosts; a member who connects with a ticket sees the shared
// projects, their sessions and files, and nothing else; unshared, the
// listener closes.
func TestSharedHostShowsTheTeamItsProjects(t *testing.T) {
	t.Setenv(proto.EnvTeamListen, "127.0.0.1:0")
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	inv, _ := db.CreateInvite(ctx, team.ID, ada.acc, api.InviteParams{Role: api.RoleMember})
	db.AcceptInvite(ctx, inv.Token, bob.acc)
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	bob.srv.learnTeams(bob.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleMember}})

	shared, private := gitRepo(t), gitRepo(t)
	os.WriteFile(filepath.Join(shared, "README.md"), []byte("the shared app\n"), 0o600)
	os.WriteFile(filepath.Join(private, "secret.md"), []byte("mine\n"), 0o600)
	for _, p := range []string{shared, private} {
		if _, err := ada.c.AddProject(p); err != nil {
			t.Fatal(err)
		}
	}
	ada.c.CreateSession(proto.SessionCreateParams{Name: "in shared", Dir: shared, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})
	ada.c.CreateSession(proto.SessionCreateParams{Name: "in private", Dir: private, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})

	sh, err := ada.c.SetHostShare(team.ID, []string{shared})
	if err != nil || sh.Addr == "" || len(sh.Teams) != 1 {
		t.Fatalf("shared: %+v %v", sh, err)
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)
	hosts, err := bob.c.TeamHosts()
	if err != nil || len(hosts) != 1 || hosts[0].Addr != sh.Addr || hosts[0].Owner != "ada" || len(hosts[0].Projects) != 1 {
		t.Fatalf("the shared host, on the member's: %+v %v", hosts, err)
	}

	// Bob's device asks the cloud for a ticket into it, and connects.
	bobDev, _ := bob.srv.cloud.store.Get()
	bc, _ := cloudclient.New(cloud.URL, bobDev.Token)
	tk, err := bc.HostTicket(ctx, team.ID, hosts[0].Device)
	if err != nil {
		t.Fatal(err)
	}
	nc, err := net.Dial("tcp", hosts[0].Addr)
	if err != nil {
		t.Fatal(err)
	}
	v, err := client.New(nc, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	if _, err := v.TeamAuth(tk.Ticket); err != nil {
		t.Fatal(err)
	}
	if list, err := v.Projects(); err != nil || len(list) != 1 || list[0].Path != shared {
		t.Fatalf("the projects a member sees: %+v %v", list, err)
	}
	if list, err := v.Sessions(); err != nil || len(list) != 1 || list[0].Name != "in shared" {
		t.Fatalf("the sessions a member sees: %+v %v", list, err)
	}
	if f, err := v.ReadProjectFile(proto.ProjectReadParams{Path: shared, File: "README.md"}); err != nil || f.Text != "the shared app\n" {
		t.Fatalf("a shared project's file: %+v %v", f, err)
	}
	if _, err := v.ReadProjectFile(proto.ProjectReadParams{Path: private, File: "secret.md"}); err == nil {
		t.Fatal("a member read a project not shared")
	}
	if _, err := v.Notes(); err == nil {
		t.Fatal("a member read the host's notes")
	}

	if _, err := ada.c.SetHostShare(team.ID, nil); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)
	if hosts, _ := bob.c.TeamHosts(); len(hosts) != 0 {
		t.Fatalf("unshared, still listed: %+v", hosts)
	}
	time.Sleep(50 * time.Millisecond)
	if c, err := net.Dial("tcp", sh.Addr); err == nil {
		c.Close()
		t.Fatal("unshared, the listener stayed open")
	}
}
