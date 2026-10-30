package ui

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// Settings → Hooks adds a hook through its dialog, tests it, lists it,
// and shows a project's own hooks for the user to allow.
func TestHooksSection(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a1, b1 := net.Pipe()
	go srv.ServeConn(a1)
	cl, err := client.New(b1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	proj := t.TempDir()
	os.MkdirAll(filepath.Join(proj, ".enowx"), 0o755)
	os.WriteFile(filepath.Join(proj, ".enowx", "hooks.json"), []byte(`[{"name":"Format","on":true,"event":"file.changed","paths":["*.go"],"action":"run","command":"gofmt -w ."}]`), 0o644)
	if _, err := cl.Hooks(proj); err != nil { // the daemon reads it, as a session there would
		t.Fatal(err)
	}

	app := New(Options{})
	posted := make(chan func(), 16)
	app.onMain = func(fn func()) { posted <- fn }
	app.client, app.agents = cl, agent.List()
	app.host.Name = "this Mac"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(900).Height(820).Background(theme.Background).TextColor(theme.Text).Children(func() { app.hooksSection(c) })
	}, 900, 820)
	frames := func(until func() bool, what string) {
		deadline := time.Now().Add(10 * time.Second)
		for !until() {
			select {
			case fn := <-posted:
				fn()
			case <-time.After(30 * time.Millisecond):
			}
			tt.Frame()
			if time.Now().After(deadline) {
				t.Fatal("never: " + what)
			}
		}
	}
	tt.Frame()
	frames(func() bool { return app.hooks.loaded }, "the hooks")
	if !hasText(tt, "has 1 hook of its own") {
		t.Fatalf("no pending project hooks: %q", tt.Texts())
	}
	if err := tt.Click("Add a hook"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	f := &app.hooks.form
	f.hook.Name, f.hook.Action, f.hook.Tool = "No push to main", proto.HookActionRun, "git push"
	f.hook.Command = `grep -q 'main' && { echo "Push to a branch, not main." >&2; exit 2; }; exit 0`
	tt.Frame()
	if !hasText(tt, "In the agent: Claude Code, Codex") {
		t.Fatalf("reach: %q", tt.Texts())
	}
	if err := tt.Click("Test"); err != nil {
		t.Fatal(err)
	}
	frames(func() bool { return f.tested != nil }, "the test")
	tt.Frame()
	shot(t, tt, "hooks-form.png")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	frames(func() bool { return !f.open && len(app.hooks.info.Hooks) == 1 }, "the saved hook")
	tt.Frame()
	if !hasText(tt, "No push to main") {
		t.Fatalf("not listed: %q", tt.Texts())
	}
	shot(t, tt, "hooks-list.png")
	if err := tt.Click("Log"); err != nil {
		t.Fatal(err)
	}
	frames(func() bool { return len(app.hooks.log) > 0 }, "the log")
	tt.Frame()
	shot(t, tt, "hooks-log.png")
}

func hasText(tt *ui.Tester, s string) bool {
	for _, x := range tt.Texts() {
		if strings.Contains(x, s) {
			return true
		}
	}
	return false
}
