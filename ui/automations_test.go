package ui

import (
	"net"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// Settings → Automations adds one through its form, on the host's
// daemon, and lists it.
func TestAutomationsSectionAddsOne(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git: %v\n%s", err, out)
	}
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
	proj, err := cl.AddProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	posted := make(chan func(), 16)
	app.onMain = func(fn func()) { posted <- fn }
	app.client, app.projects, app.project = cl, []proto.ProjectInfo{proj}, proj.Path
	app.agents = agent.List()
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(760).Height(900).Background(theme.Background).Children(func() { app.automationsSection(c) })
	}, 760, 900)
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
	frames(func() bool { return app.autos.loaded }, "the list")
	if err := tt.Click("Add an automation"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	e := app.autos.editing
	e.Name, e.Prompt, e.Schedule, e.Agent = "Weekday triage", "Triage the new issues", "weekdays 09:00", "claude"
	tt.Frame()
	shot(t, tt, "automation-form.png")
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	frames(func() bool { return app.autos.editing == nil && len(app.autos.list) == 1 }, "the saved automation")
	tt.Frame()
	if !tt.HasText("Weekday triage") || !tt.HasText("Run now") {
		t.Fatalf("list: %q", tt.Texts())
	}
	shot(t, tt, "automations.png")
	if got := app.autos.list[0]; got.Project != repo || !got.Enabled || got.NextRun.IsZero() {
		t.Fatalf("saved %+v", got)
	}
}
