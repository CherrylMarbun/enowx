package ui

import (
	"os"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// A redesign under way shows over its design: the score from where it
// started, the passes, the one under way, those done, and at the end the
// score it reached.
func TestUnslopShowsAsItGoes(t *testing.T) {
	h := newCanvasHarness(t)
	slop, err := os.ReadFile("../core/unslop/testdata/slop.html")
	if err != nil {
		t.Fatal(err)
	}
	cv, err := h.cl.CreateCanvas(proto.CanvasCreateParams{Title: "Dashboard", Kind: proto.CanvasMockup})
	if err != nil {
		t.Fatal(err)
	}
	h.cl.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: cv.Frames[0].ID, HTML: string(slop)})
	h.settle("the canvas open", func() bool { return h.app.canvases.Selected() == cv.ID })
	// As the button does, but with no agent to start.
	u, err := h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Start: true})
	if err != nil {
		t.Fatal(err)
	}
	h.app.canvases.Goto(h.app.canvasOptions(), cv.ID, u.Frame)
	h.settle("the reading", func() bool {
		return h.tt.HasText("Reading the design…") && h.tt.HasText("Spacing") && h.tt.HasText("Truth")
	})
	h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Pass: "1"})
	h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Pass: "3"})
	h.settle("pass 3 under way", func() bool { return h.tt.HasText("03 Spacing") })
	shot(t, h.tt, "canvas-unslop.png")
	h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Pass: "done"})
	h.settle("done", func() bool { return h.tt.HasText("Unslopped") })
}

// Fanned out, the panel lists the regions' agents, each its color and the
// pass it is at, checked once done.
func TestUnslopShowsItsRegions(t *testing.T) {
	h := newCanvasHarness(t)
	page := `<style>body{background:#111;color:#eee;font:700 15px Arial}</style><aside id="side"><a id="nav1">Home 🏠</a></aside><main id="main"><h1 id="title">Welcome back!!</h1></main>`
	cv, _ := h.cl.CreateCanvas(proto.CanvasCreateParams{Title: "App", Kind: proto.CanvasMockup})
	h.cl.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: cv.Frames[0].ID, HTML: page})
	h.settle("the canvas open", func() bool { return h.app.canvases.Selected() == cv.ID })
	u, err := h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Start: true})
	if err != nil {
		t.Fatal(err)
	}
	h.app.canvases.Goto(h.app.canvasOptions(), cv.ID, u.Frame)
	lead, err := h.cl.CreateSession(proto.SessionCreateParams{Agent: "shell", Name: "lead", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"1", "2", "3", "4", "5"} {
		h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Pass: p})
	}
	if _, err := h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Regions: []string{"#side", "#main"}, Session: lead.ID}); err != nil {
		t.Fatal(err)
	}
	h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Region: "side", Pass: "7"})
	h.cl.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: u.Frame, Region: "main", Pass: "done"})
	h.settle("the regions", func() bool {
		return h.tt.HasText("1 agent at work") && h.tt.HasText("2 agents at once") && h.tt.HasText("#side") && h.tt.HasText("07")
	})
	shot(t, h.tt, "canvas-unslop-regions.png")

	// A region's agent followed in the agent panel, the canvas staying
	// open; the bar goes to another, or back to the main agent.
	if err := h.tt.Click("Show the agent of #side"); err != nil {
		t.Fatal(err)
	}
	h.settle("#side's agent beside the canvas", func() bool {
		return h.app.page == pageCanvas && h.tt.HasText("Main agent") && h.app.regionViewing() != ""
	})
	h.tt.Advance(time.Second) // the panel's entrance
	h.tt.Frame()
	shot(t, h.tt, "canvas-unslop-region-agent.png")
	side := h.app.regionViewing()
	if err := h.tt.Click("Show the agent of #main"); err != nil {
		t.Fatal(err)
	}
	h.settle("#main's agent", func() bool { return h.app.regionViewing() != "" && h.app.regionViewing() != side })
	if err := h.tt.Click("Back to the main agent"); err != nil {
		t.Fatal(err)
	}
	h.settle("the main agent again", func() bool { return h.app.regionViewing() == "" && !h.tt.HasText("Main agent") })
}
