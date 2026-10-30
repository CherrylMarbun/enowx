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

// The independent review shows under way, its reviewer a click away, then
// its score and what it asks.
func TestPanelShowsTheReview(t *testing.T) {
	var p Panel
	opened := ""
	run := &proto.CanvasUnslopRun{ID: "c", Frame: "f", From: 40, Score: 90, Scans: 4, Done: true, Passes: []string{"01 Truth"},
		Review: &proto.DesignReview{Round: 1, Session: "rev1"}}
	tt := ui.NewTester(func(c *ui.Context) {
		p.Draw(c, run, PanelOptions{Reading: "Reading…", OpenReviewer: func(id string) { opened = id }})
	}, 900, 800)
	tt.Frame()
	if err := tt.Click("Show the reviewer"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	if tt.Frame(); opened != "rev1" {
		t.Fatalf("opened %q", opened)
	}
	run.Review = &proto.DesignReview{Round: 1, Session: "rev1", Done: true, Score: 6, Issues: []string{"hero: two CTAs → one"}, Keep: "the type"}
	tt.Frame()
	if !tt.HasText("6/10") || !tt.HasText("· hero: two CTAs → one") || !tt.HasText("Keep: the type") {
		t.Fatalf("the verdict: %q", tt.Texts())
	}
}
