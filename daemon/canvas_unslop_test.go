package daemon

import (
	"os"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Unslop reads a mockup's design; started, it copies the design right
// after it, reads the copy, and says what to ask the canvas's agent, the
// original left as it was.
func TestCanvasUnslop(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	slop, err := os.ReadFile("../core/unslop/testdata/slop.html")
	if err != nil {
		t.Fatal(err)
	}
	cv, err := c.CreateCanvas(proto.CanvasCreateParams{Title: "Dashboard", Kind: proto.CanvasMockup})
	if err != nil {
		t.Fatal(err)
	}
	orig := cv.Frames[0]
	if _, err := c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: orig.ID, HTML: string(slop)}); err != nil {
		t.Fatal(err)
	}
	read, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID})
	if err != nil || read.Frame != orig.ID || read.Score > 20 || len(read.Findings) < 10 || read.Ask != "" {
		t.Fatalf("read: %+v %v", read, err)
	}
	started, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: "1", Start: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := c.Canvas(cv.ID)
	if len(got.Frames) != 2 || got.Frames[1].ID != started.Frame || !strings.HasSuffix(got.Frames[1].Name, "· unslop") || got.Frames[0].HTML != string(slop) {
		t.Fatalf("after starting: %d designs, the copy %+v", len(got.Frames), got.Frames[len(got.Frames)-1].Name)
	}
	if !strings.Contains(started.Ask, "canvas unslop "+cv.ID+" --frame "+started.Frame) || !strings.Contains(started.Ask, "compare "+cv.ID+" 1 2") {
		t.Fatalf("the ask: %s", started.Ask)
	}
	// A board's shapes have no HTML to read.
	board, _ := c.CreateCanvas(proto.CanvasCreateParams{Title: "Board", Kind: proto.CanvasBoard})
	if _, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: board.ID}); err == nil {
		t.Fatal("a board was read")
	}
}
