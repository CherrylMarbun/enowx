package daemon

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// Unslop reads a mockup's design; started, it copies the design right
// after it, reads the copy, and says what to ask the canvas's agent, the
// original left as it was.
func TestCanvasUnslop(t *testing.T) {
	srv := startServer(t)
	runs := make(chan proto.CanvasUnslopRun, 8)
	connect(t, srv, func(e proto.Event) {
		var r proto.CanvasUnslopRun
		if e.Name == proto.EventCanvasUnslop && json.Unmarshal(e.Data, &r) == nil {
			runs <- r
		}
	})
	c := connect(t, srv, nil)
	next := func() proto.CanvasUnslopRun {
		t.Helper()
		select {
		case r := <-runs:
			return r
		case <-time.After(5 * time.Second):
			t.Fatal("no run event")
		}
		return proto.CanvasUnslopRun{}
	}
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
	if !strings.Contains(started.Ask, `canvas_unslop (canvas "`+cv.ID+`", frame "`+started.Frame+`")`) || !strings.Contains(started.Ask, "designs 1 and 2") {
		t.Fatalf("the ask: %s", started.Ask)
	}
	// The run as it goes: started, a pass begun, the next, done.
	if r := next(); r.Frame != started.Frame || r.Original != got.Frames[0].ID || r.From != started.Score || len(r.Passes) != 12 || r.Pass != "" || r.Scans != 1 {
		t.Fatalf("started: %+v", r)
	}
	// A pass waits for the brief, kept on the canvas and in the run.
	if _, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: started.Frame, Pass: "1"}); err == nil || !strings.Contains(err.Error(), "canvas_brief") {
		t.Fatalf("a pass before the brief: %v", err)
	}
	brief := proto.DesignBrief{Product: "A team's dashboard", Users: "Leads checking the week", Tone: "Calm, dense", FirstScreen: "This week's numbers"}
	if _, err := c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpBrief, Brief: &brief}); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct{ pass, want string }{{"3", "03 Spacing"}, {"type", "04 Type"}, {"done", ""}} {
		if _, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: started.Frame, Pass: step.pass}); err != nil {
			t.Fatal(err)
		}
		if r := next(); r.Pass != step.want || r.Brief == nil || *r.Brief != brief {
			t.Fatalf("pass %s: %+v", step.pass, r.Pass)
		} else if step.pass == "done" && (!r.Done || len(r.Finished) != 2 || r.Finished[0] != "03 Spacing") {
			t.Fatalf("done: %+v", r)
		}
	}
	if _, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: started.Frame, Pass: "13"}); err == nil {
		t.Fatal("a pass that isn't was taken")
	}
	// The copy turned light, from the dark original: found, first.
	started2, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: "1", Start: true})
	if err != nil {
		t.Fatal(err)
	}
	if r := next(); r.Theme != "dark" {
		t.Fatalf("the original's theme: %q", r.Theme)
	}
	light := strings.Replace(string(slop), "body{margin:0;", "body{background:#ffffff;margin:0;", 1)
	light = strings.Replace(light, "background:linear-gradient(135deg,#4f46e5,#9333ea);", "", 1)
	if _, err := c.EditCanvas(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: started2.Frame, HTML: light}); err != nil {
		t.Fatal(err)
	}
	got2, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: started2.Frame})
	if err != nil || len(got2.Findings) == 0 || got2.Findings[0].Rule != "theme-changed" {
		t.Fatalf("a turned theme: %+v %v", got2.Findings, err)
	}
	if !strings.Contains(started2.Ask, "dark design and stays dark") {
		t.Fatalf("the ask doesn't keep the theme: %s", started2.Ask)
	}
	// A board's shapes have no HTML to read.
	board, _ := c.CreateCanvas(proto.CanvasCreateParams{Title: "Board", Kind: proto.CanvasBoard})
	if _, err := c.CanvasUnslop(proto.CanvasUnslopParams{ID: board.ID}); err == nil {
		t.Fatal("a board was read")
	}
}
