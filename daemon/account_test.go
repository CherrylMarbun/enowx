package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A session of another account runs its agent with that account's home,
// which shares the agent's settings but not its sign-in, and keeps it, with
// the preset's variables, when continued.
func TestAccountsGiveAgentsAHomeOfTheirOwn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	claudeHome := filepath.Join(home, ".claude")
	for name, content := range map[string]string{"settings.json": "{}", ".credentials.json": "secret"} {
		if err := os.MkdirAll(claudeHome, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(claudeHome, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"home=$CLAUDE_CONFIG_DIR teams=$CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS\"\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	srv := startServer(t)
	c := connect(t, srv, nil)
	if _, err := c.AddAccount("aider", "work"); err == nil {
		t.Fatal("an account of an agent that can't have one")
	}
	acc, err := c.AddAccount("claude", "work")
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := c.Accounts(); len(list) != 1 || list[0].Name != "work" {
		t.Fatalf("accounts %+v", list)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude-agent-teams", Account: acc.ID, Dir: t.TempDir(), Cols: 120, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	accHome := srv.accounts.Home(acc.ID)
	sess, _ := srv.sessions.Get(info.ID)
	want := "home=" + accHome + " teams=1"
	waitFor(t, "the account's home", func() bool { return strings.Contains(sess.Terminal().Text(), want) })
	if info.Account != acc.ID {
		t.Fatalf("session's account %q", info.Account)
	}
	if _, err := os.Stat(filepath.Join(accHome, "settings.json")); err != nil {
		t.Fatalf("settings not shared: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(accHome, ".credentials.json")); err == nil {
		t.Fatal("the default account's sign-in is shared")
	}

	// Stopped and continued, it runs as the same account, as Agent Teams.
	if err := srv.hibernateSession(sess); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ContinueSession(info.ID); err != nil {
		t.Fatal(err)
	}
	sess, _ = srv.sessions.Get(info.ID)
	waitFor(t, "the account when continued", func() bool {
		return strings.Count(sess.Terminal().Text(), want) >= 2
	})

	if err := c.RemoveAccount(acc.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(accHome); err == nil {
		t.Fatal("the account's home stayed")
	}
}
