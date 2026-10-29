//go:build !windows

package ui

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A project's rules, written from its menu's dialog, are what its
// sessions read.
func TestProjectRulesAreWrittenFromTheDialog(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	app := New(Options{})
	var mu sync.Mutex
	var later []func()
	app.onMain = func(fn func()) { mu.Lock(); later = append(later, fn); mu.Unlock() }
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, func(ev proto.Event) { app.handleEvent(1, ev) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	app.gen, app.client = 1, cl
	app.host = hosts.Host{ID: hosts.LocalID, Name: "this Mac"}
	project := proto.ProjectInfo{Path: t.TempDir(), Name: "app"}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(900).Height(700).Background(theme.Background).TextColor(theme.Text).Children(func() { app.rulesDialog(c) })
	}, 900, 700)
	settle := func(what string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			tt.Frame()
			if done() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			fns := later
			later = nil
			mu.Unlock()
			for _, fn := range fns {
				fn()
			}
		}
	}
	app.openRules(project)
	settle("the dialog, read", func() bool { return app.rulesEditing != nil && app.rulesEditing.loaded && tt.HasText("Rules of app") })
	app.rulesEditing.text = "- Run the tests first"
	tt.Frame()
	shot(t, tt, "project-rules.png")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	settle("saved", func() bool { return app.rulesEditing == nil || !app.rulesEditing.open })
	if r, err := cl.ProjectRules(project.Path); err != nil || r.Rules != "- Run the tests first\n" {
		t.Fatalf("the rules kept: %+v %v", r, err)
	}
}
