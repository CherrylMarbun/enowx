package daemon

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A member starts a session on another member's host, in a shared
// project: it runs as an agent account of their own there, with their git
// identity, theirs to type into; not in a project not shared, not as a
// viewer.
func TestMemberStartsASessionWithTheirOwnAccount(t *testing.T) {
	t.Setenv(proto.EnvTeamListen, "127.0.0.1:0")
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o700)
	os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte("the host's own sign-in"), 0o600)
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"home=$CLAUDE_CONFIG_DIR who=$GIT_AUTHOR_NAME <$GIT_AUTHOR_EMAIL>\"\nsleep 30\n"
	os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
	shared, private := gitRepo(t), gitRepo(t)
	ada.c.AddProject(shared)
	ada.c.AddProject(private)
	sh, _ := ada.c.SetHostShare(team.ID, []string{shared})

	adaDev, _ := ada.srv.cloud.store.Get()
	bobDev, _ := bob.srv.cloud.store.Get()
	bc, _ := cloudclient.New(cloud.URL, bobDev.Token)
	tk, _ := bc.HostTicket(ctx, team.ID, adaDev.DeviceID)
	nc, err := net.Dial("tcp", sh.Addr)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := client.New(nc, nil)
	t.Cleanup(func() { v.Close() })
	if _, err := v.TeamAuth(tk.Ticket); err != nil {
		t.Fatal(err)
	}

	if _, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: private, Cols: 100, Rows: 10}); err == nil {
		t.Fatal("a member started a session in a project not shared")
	}
	if _, err := v.CreateSession(proto.SessionCreateParams{Project: shared, Cols: 100, Rows: 10}); err == nil {
		t.Fatal("a member started a shell on the host")
	}
	info, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: shared, Name: "bob's work", Cols: 100, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	if info.Member == nil || info.Member.GitHub != "bob" || info.Account == "" {
		t.Fatalf("the member's session: %+v", info)
	}
	accHome := ada.srv.accounts.Home(info.Account)
	sess, _ := ada.srv.sessions.Get(info.ID)
	waitFor(t, "the member's account and identity", func() bool {
		return strings.Contains(sess.Terminal().Text(), "home="+accHome+" who=bob <bob@users.noreply.github.com>")
	})
	if data, _ := os.ReadFile(filepath.Join(accHome, ".credentials.json")); len(data) > 0 {
		t.Fatal("the member's account has the host's sign-in")
	}
	if list, _ := ada.c.Accounts(); len(list) != 1 || list[0].Name != "Team · @bob" {
		t.Fatalf("the host's accounts: %+v", list)
	}
	if !ada.srv.ownsSession(info.ID, info.Member.Account) {
		t.Fatal("the member can't type into their own session")
	}
}
