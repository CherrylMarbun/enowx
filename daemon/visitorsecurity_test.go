package daemon

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// visitWorld is a host (ada) sharing one project of two with a team, and
// bob, its member with a role, able to visit.
type visitWorld struct {
	db              *store.DB
	cloudURL        string
	ada, bob        teamHost
	team            string
	shared, private string
	addr            string
}

func newVisitWorld(t *testing.T, role string, allowSessions bool) *visitWorld {
	t.Helper()
	t.Setenv(proto.EnvTeamListen, "127.0.0.1:0")
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	w := &visitWorld{db: db, cloudURL: cloud.URL}
	w.ada = signInHost(t, db, cloud.URL, 1, "ada")
	w.bob = signInHost(t, db, cloud.URL, 2, "bob")
	team, _ := db.CreateTeam(ctx, w.ada.acc, "Design")
	w.team = team.ID
	inv, _ := db.CreateInvite(ctx, team.ID, w.ada.acc, api.InviteParams{Role: role})
	if _, err := db.AcceptInvite(ctx, inv.Token, w.bob.acc); err != nil {
		t.Fatal(err)
	}
	w.ada.srv.learnTeams(w.ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	w.shared, w.private = gitRepo(t), gitRepo(t)
	w.ada.c.AddProject(w.shared)
	w.ada.c.AddProject(w.private)
	sh, err := w.ada.c.SetHostShareTeam(proto.HostShareTeam{Team: team.ID, Projects: []string{w.shared}, AllowSessions: &allowSessions})
	if err != nil || sh.Addr == "" {
		t.Fatalf("share: %+v %v", sh, err)
	}
	w.addr = sh.Addr
	return w
}

// ticket is a fresh ticket of bob's into ada's host.
func (w *visitWorld) ticket(t *testing.T) string {
	t.Helper()
	adaDev, _ := w.ada.srv.cloud.store.Get()
	bobDev, _ := w.bob.srv.cloud.store.Get()
	bc, _ := cloudclient.New(w.cloudURL, bobDev.Token)
	tk, err := bc.HostTicket(context.Background(), w.team, adaDev.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	return tk.Ticket
}

// visit connects bob through the team listener with a fresh ticket.
func (w *visitWorld) visit(t *testing.T) *client.Client {
	t.Helper()
	tk := w.ticket(t)
	nc, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	v, err := client.New(nc, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	if _, err := v.TeamAuth(tk); err != nil {
		t.Fatal(err)
	}
	return v
}

// fakeClaude puts a "claude" on PATH that runs script, then waits.
func fakeClaude(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"+script+"\nsleep 30\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A member's own session gets nothing of the host's: not the daemon's own
// environment, the variables its user set for sessions, its model API key, the program its hooks
// reach the daemon with, a variable the visitor passes, nor anything of
// the host's user's agent home (their conversations, settings).
func TestMemberSessionGetsNoHostSecrets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude", "projects", "-private-repo"), 0o700)
	os.WriteFile(filepath.Join(home, ".claude", ".credentials.json"), []byte("host sign-in"), 0o600)
	os.WriteFile(filepath.Join(home, ".claude", "projects", "-private-repo", "c.jsonl"), []byte("HOST_PRIVATE_CONVERSATION\n"), 0o600)
	os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"env":{"X":"HOST_SETTINGS"}}`), 0o600)
	os.MkdirAll(filepath.Join(home, ".enowx-app", "agentapi"), 0o700)
	os.WriteFile(filepath.Join(home, ".enowx-app", "agentapi", "claude.env"), []byte("ANTHROPIC_API_KEY=sk-HOST-API-KEY\n"), 0o600)
	t.Setenv("HOST_DAEMON_TOKEN", "daemon-env-secret")
	fakeClaude(t, `echo "tok=$HOST_SECRET_TOKEN key=$ANTHROPIC_API_KEY inj=$BOB_INJECTED bin=$ENOWX_BIN. daemon=$HOST_DAEMON_TOKEN"; cat "$CLAUDE_CONFIG_DIR"/projects/*/*.jsonl "$CLAUDE_CONFIG_DIR"/settings.json; echo END`)

	w := newVisitWorld(t, api.RoleMember, true)
	w.ada.srv.HookBin = "/host/enowx"
	if _, err := w.ada.c.SetSessionEnv("HOST_SECRET_TOKEN", "ghp_HOSTSECRET"); err != nil {
		t.Fatal(err)
	}
	v := w.visit(t)
	info, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: w.shared, Cols: 200, Rows: 20,
		Env: []string{"BOB_INJECTED=yes"}})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := w.ada.srv.sessions.Get(info.ID)
	waitFor(t, "the member session's output", func() bool { return strings.Contains(sess.Terminal().Text(), "END") })
	text := sess.Terminal().Text()
	for what, leaked := range map[string]bool{
		"the host's session variable":   strings.Contains(text, "ghp_HOSTSECRET"),
		"the host's model API key":      strings.Contains(text, "sk-HOST-API-KEY"),
		"the daemon's own environment":  strings.Contains(text, "daemon-env-secret"),
		"a variable the visitor passed": strings.Contains(text, "inj=yes"),
		"the hooks' program":            !strings.Contains(text, "bin=."),
		"the host's conversations":      strings.Contains(text, "HOST_PRIVATE_CONVERSATION"),
		"the host's ~/.claude/settings": strings.Contains(text, "HOST_SETTINGS"),
	} {
		if leaked {
			t.Errorf("a member's session got %s:\n%s", what, text)
		}
	}
	entries, _ := os.ReadDir(w.ada.srv.accounts.Home(info.Account))
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			t.Errorf("the member's agent home links %s", e.Name())
		}
	}
}

// Members start sessions only once the host's user lets them, per team;
// changing the projects shared keeps the setting, unsharing resets it.
func TestMemberSessionsNeedTheShareToAllowThem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	fakeClaude(t, `echo started`)
	w := newVisitWorld(t, api.RoleMember, false)
	v := w.visit(t)
	if _, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: w.shared, Cols: 100, Rows: 10}); err == nil {
		t.Fatal("a member started a session, the share not letting them")
	}
	on := true
	if _, err := w.ada.c.SetHostShareTeam(proto.HostShareTeam{Team: w.team, Projects: []string{w.shared}, AllowSessions: &on}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: w.shared, Cols: 100, Rows: 10}); err != nil {
		t.Fatalf("let, a member's session: %v", err)
	}
	sh, err := w.ada.c.SetHostShare(w.team, []string{w.shared, w.private})
	if err != nil || len(sh.Teams) != 1 || sh.Teams[0].AllowSessions == nil || !*sh.Teams[0].AllowSessions {
		t.Fatalf("projects changed, sessions still let: %+v %v", sh, err)
	}
	w.ada.c.SetHostShare(w.team, nil)
	sh, _ = w.ada.c.SetHostShare(w.team, []string{w.shared})
	if len(sh.Teams) != 1 || *sh.Teams[0].AllowSessions {
		t.Fatalf("shared again, sessions start off: %+v", sh)
	}
}

// A member's session on a new branch runs no setup script of the
// project's: the host would run it with nobody's account.
func TestMemberSessionRunsNoSetup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	fakeClaude(t, `echo started`)
	w := newVisitWorld(t, api.RoleMember, true)
	os.WriteFile(filepath.Join(w.shared, ".enowx.toml"), []byte("[worktree]\nsetup = \"echo SETUP_RAN\"\n"), 0o644)
	v := w.visit(t)
	if _, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: w.shared, BranchMode: proto.BranchNew,
		Branch: "bob/x", RunSetup: true, Cols: 100, Rows: 10}); err == nil {
		t.Fatal("a member's session ran the project's setup")
	}
	for _, s := range w.ada.srv.sessions.List() {
		if s.Info().Name == "Setup" {
			t.Fatalf("a setup tab started: %+v", s.Info())
		}
	}
}

// A member's agent account is found by their cloud account, not their
// login: another account with the same login gets one of its own, and the
// same member renamed keeps theirs.
func TestMemberAccountIsKeptByCloudAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv := startServer(t)
	a, err := srv.memberAccount("claude", &proto.Visitor{Account: "acc-original", GitHub: "bob", Name: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := srv.memberAccount("claude", &proto.Visitor{Account: "acc-someone-else", GitHub: "BOB", Name: "Mallory"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatalf("two cloud accounts share agent account %s (%q)", a.ID, a.Name)
	}
	again, err := srv.memberAccount("claude", &proto.Visitor{Account: "acc-original", GitHub: "bobby", Name: "Bob"})
	if err != nil || again.ID != a.ID {
		t.Fatalf("the same member, renamed: %+v %v (was %s)", again, err, a.ID)
	}
}
