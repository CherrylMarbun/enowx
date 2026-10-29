package ui

import (
	"os"
	"testing"

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
