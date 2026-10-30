package unslopview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// A run's brief shows in a line, and in full once clicked.
func TestPanelShowsTheBrief(t *testing.T) {
	var p Panel
	run := &proto.CanvasUnslopRun{ID: "c", Frame: "f", From: 40, Score: 52, Scans: 2, Passes: []string{"01 Truth"},
		Brief: &proto.DesignBrief{Product: "A read-only API's docs", Users: "Developers wiring a client", Tone: "Calm, dense", FirstScreen: "The endpoints"}}
	tt := ui.NewTester(func(c *ui.Context) { p.Draw(c, run, PanelOptions{Reading: "Reading…"}) }, 900, 700)
	tt.Frame()
	if !tt.HasText("A read-only API's docs · Calm, dense") || tt.HasText("Developers wiring a client") {
		t.Fatalf("the brief in a line: %q", tt.Texts())
	}
	if err := tt.Click("The brief: what it is for"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Developers wiring a client") || !tt.HasText("First screen") {
		t.Fatalf("the brief in full: %q", tt.Texts())
	}
}
