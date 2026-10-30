package daemon

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/designcompare"
	"github.com/enowx/enowx/proto"
)

// A UI build: started from the app, its ask says what to do; asked to
// explore, fresh agents build a direction each, on a design only they may
// change, a card before done; once all are, each pair's distinctness is
// on the run and the stage is compare; the lead's done has it reviewed.
func TestCanvasBuild(t *testing.T) {
	srv := startServer(t)
	runs := make(chan proto.CanvasBuildRun, 32)
	c := connect(t, srv, func(ev proto.Event) {
		var run proto.CanvasBuildRun
		if ev.Name == proto.EventCanvasBuild && json.Unmarshal(ev.Data, &run) == nil {
			runs <- run
		}
	})
	until := func(what string, ok func(proto.CanvasBuildRun) bool) proto.CanvasBuildRun {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case r := <-runs:
				if ok(r) {
					return r
				}
			case <-deadline:
				t.Fatalf("no run %s", what)
			}
		}
	}
	cv, _ := c.CreateCanvas(proto.CanvasCreateParams{Title: "Ledger", Kind: proto.CanvasMockup})
	if _, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Start: true}); err == nil {
		t.Fatal("a build of nothing was started")
	}
	b, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Start: true, What: "A pricing page", Directions: 3, Confirm: true})
	if err != nil || b.Run.Want != 3 || b.Run.Stage != proto.BuildBrief || !strings.Contains(b.Ask, "fanout 3") || !strings.Contains(b.Ask, "canvas_brief") {
		t.Fatalf("started: %+v %v\n%s", b.Run, err, b.Ask)
	}
	lead, err := c.CreateSession(proto.SessionCreateParams{Agent: "shell", Name: "lead", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Fanout: 2, Session: lead.ID}); err == nil || !strings.Contains(err.Error(), "brief") {
		t.Fatalf("fanned out before the brief: %v", err)
	}
	c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpBrief, Brief: &proto.DesignBrief{Product: "Pricing for a ledger app", Users: "Small shops", Tone: "Calm", FirstScreen: "The plans"}})
	if _, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Fanout: 2, Session: lead.ID}); err != nil {
		t.Fatal(err)
	}
	run := until("with two directions", func(r proto.CanvasBuildRun) bool { return len(r.Directions) == 2 })
	if run.Stage != proto.BuildBuilding || run.Lead != lead.ID {
		t.Fatalf("building: %+v", run)
	}
	got, _ := c.Canvas(cv.ID)
	a, bdir := run.Directions[0], run.Directions[1]
	if len(got.Frames) != 3 || got.Frames[1].ID != a.Frame || got.Frames[1].Name != "Direction A" {
		t.Fatalf("designs: %+v", got.Frames)
	}
	// Each changes its own design only.
	if _, err := c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: bdir.Frame, HTML: "<p>x</p>", Session: a.Session}); err == nil {
		t.Fatal("a direction changed another's design")
	}
	if _, err := c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: a.Frame, HTML: "<main>A</main>", Session: a.Session}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Done: true, Session: a.Session}); err == nil || !strings.Contains(err.Error(), "card") {
		t.Fatalf("done without a card: %v", err)
	}
	cards := map[string]string{
		a.Frame:    "North star: a ledger you trust\nSource: ruled paper\nSkeleton: full-bleed type over the plans\nType: Fraunces 300\nPalette: #f4efe6, #c2410c",
		bdir.Frame: "North star: a shop till\nSource: a receipt printer\nSkeleton: a narrow receipt column centered\nType: IBM Plex Mono 400\nPalette: #111111, #22c55e",
	}
	for _, d := range run.Directions {
		card, _ := designcompare.ParseCard(cards[d.Frame])
		if _, err := c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpDirection, Frame: d.Frame, Direction: &card, Session: d.Session}); err != nil {
			t.Fatal(err)
		}
		if _, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Done: true, Session: d.Session}); err != nil {
			t.Fatal(err)
		}
	}
	run = until("to compare", func(r proto.CanvasBuildRun) bool { return r.Stage == proto.BuildCompare })
	if len(run.Pairs) != 1 || run.Pairs[0].Distinct < 90 || len(run.Pairs[0].Shared) != 0 {
		t.Fatalf("pairs: %+v", run.Pairs)
	}
	if _, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Done: true, Frame: "2", Session: lead.ID}); err != nil {
		t.Fatal(err)
	}
	until("reviewed", func(r proto.CanvasBuildRun) bool { return r.Stage == proto.BuildReviewing })
}

// A build's run lives in memory: after a restart the lead's done takes it
// up again from the canvas and has the design reviewed.
func TestCanvasBuildAfterARestart(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	cv, _ := c.CreateCanvas(proto.CanvasCreateParams{Title: "Ledger", Kind: proto.CanvasMockup})
	b, err := c.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Done: true, Frame: "1"})
	if err != nil || b.Run == nil || b.Run.Stage != proto.BuildReviewing {
		t.Fatalf("done with no run: %+v %v", b.Run, err)
	}
}
