package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// A session's account hits its usage limit: by the preferences it goes
// on as the next account, its conversation resumed and told to go on;
// asked first, the user is asked; once the limit reset, it goes back.
func TestAccountFallsBack(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	srv := startServer(t)
	falls := make(chan proto.AccountFallback, 8)
	c := connect(t, srv, func(e proto.Event) {
		var f proto.AccountFallback
		if e.Name == proto.EventAccountFallback && json.Unmarshal(e.Data, &f) == nil {
			falls <- f
		}
	})
	work, _ := c.AddAccount("claude", "Work")
	spare, _ := c.AddAccount("claude", "Spare")
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"home=$CLAUDE_CONFIG_DIR\"\ncase \"$CLAUDE_CONFIG_DIR\" in *" + work.ID + "*) echo 'Claude usage limit reached. Your limit will reset at 3pm';; esac\nwhile read -r line; do echo \"heard: $line\"; done\n"
	os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := srv.setAccountPrefs(proto.AccountPrefsParams{Agent: "claude", Prefs: proto.AccountPrefs{Order: []string{"Work", "Spare", ""}, Fallback: proto.FallbackAuto, Return: true}}); err != nil {
		t.Fatal(err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", AccountAuto: true, Dir: t.TempDir(), Cols: 120, Rows: 20})
	if err != nil {
		t.Fatal(err)
	}
	if info.Account != "" {
		t.Fatalf("no default set: %q", info.Account)
	}
	srv.setAccountPrefs(proto.AccountPrefsParams{Agent: "claude", Prefs: proto.AccountPrefs{Order: []string{"Work", "Spare", ""}, Fallback: proto.FallbackAuto, Return: true, Default: "Work"}})
	info, err = c.CreateSession(proto.SessionCreateParams{Agent: "claude", AccountAuto: true, Dir: t.TempDir(), Cols: 120, Rows: 20})
	if err != nil || info.Account != work.ID {
		t.Fatalf("default Work: %q %v", info.Account, err)
	}
	var f proto.AccountFallback
	select {
	case f = <-falls:
	case <-time.After(15 * time.Second):
		t.Fatal("no fallback")
	}
	if !f.Switched || f.From != "Work" || f.To != "Spare" || f.Reason != proto.AccountLimit || f.Until.Hour() != 15 {
		t.Fatalf("fallback: %+v", f)
	}
	sess, _ := srv.sessions.Get(info.ID)
	waitFor(t, "going on as Spare, told to", func() bool {
		text := sess.Terminal().Text()
		return strings.Contains(text, "home="+srv.accounts.Home(spare.ID)) && strings.Contains(text, "heard: ")
	})
	if got, _ := srv.sessions.Get(info.ID); got.Info().Account != spare.ID {
		t.Fatalf("account %q", got.Info().Account)
	}
	log := srv.accountLog()
	if len(log) < 2 || log[0].Event != "fallback" || log[0].Account != "Spare" {
		t.Fatalf("log: %+v", log[:min(3, len(log))])
	}
	st := srv.accountsInfo()
	for _, a := range st.Accounts {
		if a.Name == "Work" && a.State != proto.AccountLimit {
			t.Fatalf("Work's state: %+v", a)
		}
	}

	// Its limit reset: back to Work.
	srv.goBack(time.Now().Add(24 * time.Hour))
	select {
	case f = <-falls:
	case <-time.After(10 * time.Second):
		t.Fatal("never went back")
	}
	if !f.Back || f.To != "Work" {
		t.Fatalf("back: %+v", f)
	}
}

// Asked first, a limit waits for the user; switched by hand, the session
// goes on as the account picked.
func TestAccountFallbackAsks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	srv := startServer(t)
	falls := make(chan proto.AccountFallback, 8)
	c := connect(t, srv, func(e proto.Event) {
		var f proto.AccountFallback
		if e.Name == proto.EventAccountFallback && json.Unmarshal(e.Data, &f) == nil {
			falls <- f
		}
	})
	spare, _ := c.AddAccount("claude", "Spare")
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"home=$CLAUDE_CONFIG_DIR\"\n[ -n \"$CLAUDE_CONFIG_DIR\" ] || echo 'Credit balance is too low'\nsleep 30\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	srv.setAccountPrefs(proto.AccountPrefsParams{Agent: "claude", Prefs: proto.AccountPrefs{Fallback: proto.FallbackAsk}})
	info, _ := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 120, Rows: 20})
	var f proto.AccountFallback
	select {
	case f = <-falls:
	case <-time.After(15 * time.Second):
		t.Fatal("not asked")
	}
	if !f.Ask || f.Switched || f.ToID != spare.ID || f.Reason != proto.AccountCredits {
		t.Fatalf("asked: %+v", f)
	}
	if got, _ := srv.sessions.Get(info.ID); got.Info().Account != "" {
		t.Fatal("switched without asking")
	}
	if _, err := c.SwitchAccount(proto.AccountSwitchParams{Session: info.ID, Account: f.ToID}); err != nil {
		t.Fatal(err)
	}
	sess, _ := srv.sessions.Get(info.ID)
	waitFor(t, "as Spare", func() bool { return strings.Contains(sess.Terminal().Text(), "home="+srv.accounts.Home(spare.ID)) })
}
