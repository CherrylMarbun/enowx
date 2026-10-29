//go:build !windows

package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// The side panel's tools sit in groups: a group shows its tools, and
// picking it again brings back the tool it showed last.
func TestSidePanelGroupsItsTools(t *testing.T) {
	app := New(Options{})
	app.state.Panel.Open, app.state.Panel.Tab = true, panelChanges
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(600).Height(200).Background(theme.Background).TextColor(theme.Text).Children(func() { app.panelStrip(c) })
	}, 600, 200)
	tt.Frame()
	if !tt.HasText("Code") || !tt.HasText("Preview") || !tt.HasText("Debug") {
		t.Fatalf("the groups: %q", tt.Texts())
	}
	if _, ok := tt.Find("Files"); !ok {
		t.Fatal("Code's tools don't show")
	}
	if _, ok := tt.Find("Traffic"); ok {
		t.Fatal("Debug's tools show under Code")
	}
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	click("Files")
	click("Debug")
	if app.state.Panel.Tab != panelTraffic {
		t.Fatalf("Debug opens on %q", app.state.Panel.Tab)
	}
	click("Activity")
	click("Code")
	if app.state.Panel.Tab != panelFiles {
		t.Fatalf("Code doesn't come back on Files: %q", app.state.Panel.Tab)
	}
	click("Debug")
	if app.state.Panel.Tab != panelActivity {
		t.Fatalf("Debug doesn't come back on Activity: %q", app.state.Panel.Tab)
	}
}
