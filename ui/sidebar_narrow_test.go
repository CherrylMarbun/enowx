package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// The Sessions panel folds to icons and back: folded, every session is
// still there by its full name (its tooltip and label), a click selects
// it, and the fold is kept with the app's state.
func TestSessionsPanelFoldsToIcons(t *testing.T) {
	app := New(Options{})
	app.projects = []proto.ProjectInfo{{Name: "cryptgo", Path: "/p/cryptgo"}, {Name: "enxapi", Path: "/p/enxapi"}}
	app.sessions = []proto.SessionInfo{
		{ID: "s1", Name: "Refactor the payment webhooks", Agent: "claude", Project: "/p/cryptgo", Branch: "main"},
		{ID: "s2", Name: "ai", Agent: "codex", Project: "/p/cryptgo", Exited: true},
	}
	app.project = "/p/cryptgo"
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Row(c).Width(400).Height(500).Background(theme.Background).Children(func() { app.sessionList(c) })
	}, 400, 500)
	tt.Frame()
	shot(t, tt, "sessions-wide.png")
	if err := tt.Click("Collapse Sessions to icons"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if !app.state.SessionsNarrow || tt.HasText("Filter sessions") {
		t.Fatalf("not folded: %v %q", app.state.SessionsNarrow, tt.Texts())
	}
	r, ok := tt.Find("Refactor the payment webhooks")
	if !ok || r.W > sessionsNarrow {
		t.Fatalf("the session isn't there folded, by its name: %v %v", ok, r)
	}
	if err := tt.Click("Refactor the payment webhooks"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if app.selected != "s1" {
		t.Fatalf("selected %q", app.selected)
	}
	// As it shows once its width moved (the tester doesn't run the motion).
	folded := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Row(c).Width(400).Height(500).Background(theme.Background).Children(func() { app.sessionList(c) })
	}, 400, 500)
	folded.Frame()
	shot(t, folded, "sessions-narrow.png")
	if r, _ := folded.Find("Expand Sessions"); r.X+r.W > sessionsNarrow {
		t.Fatalf("folded, the panel is wider than %d: %+v", sessionsNarrow, r)
	}
	if err := tt.Click("Expand Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if app.state.SessionsNarrow || !tt.HasText("Filter sessions") {
		t.Fatalf("not unfolded: %q", tt.Texts())
	}
}
