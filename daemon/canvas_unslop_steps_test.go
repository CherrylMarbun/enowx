package daemon

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A redesign keeps the design at each pass's end, labels the canvas's
// version with the pass, plays back as documents, and leaves a step out
// in a copy; a run started with only some passes runs those.
func TestUnslopSteps(t *testing.T) {
	srv := startServer(t)
	st, _ := srv.canvasesIn("")
	cv, err := st.Create(proto.CanvasCreateParams{Title: "Shop", Kind: proto.CanvasMockup})
	if err != nil {
		t.Fatal(err)
	}
	set := func(frame, html string) {
		t.Helper()
		if _, err := srv.canvasEdit(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpHTMLSet, Frame: frame, HTML: html}); err != nil {
			t.Fatal(err)
		}
	}
	set(cv.Frames[0].ID, `<style>.c{padding:13px}</style><h1 id="t" style="font-weight:800">Welcome back!! 👋</h1><p id="p">Supercharge your sales</p>`)
	brief := proto.DesignBrief{Product: "A shop", Users: "Owners", Tone: "Calm", FirstScreen: "Orders"}
	if _, err := srv.canvasEdit(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpBrief, Brief: &brief}); err != nil {
		t.Fatal(err)
	}
	started, err := srv.canvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Start: true, Only: []string{"3", "copy"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(started.Ask, "Run only these passes, in this order, and skip the others: 03 Spacing, 08 Copy.") || len(started.Run.Passes) != 2 {
		t.Fatalf("only: %+v %s", started.Run.Passes, started.Ask)
	}
	f := started.Frame
	pass := func(p string) proto.CanvasUnslopRun {
		t.Helper()
		out, err := srv.canvasUnslop(proto.CanvasUnslopParams{ID: cv.ID, Frame: f, Pass: p})
		if err != nil {
			t.Fatal(err)
		}
		return *out.Run
	}
	pass("3")
	set(f, `<style>.c{padding:16px}</style><h1 id="t" style="font-weight:800">Welcome back!! 👋</h1><p id="p">Supercharge your sales</p>`)
	if r := pass("8"); len(r.Passed) != 1 || len(r.Passed[0].Passes) != 1 || r.Passed[0].Passes[0] != "03 Spacing" {
		t.Fatalf("after spacing: %+v", r.Passed)
	}
	set(f, `<style>.c{padding:16px}</style><h1 id="t" style="font-weight:500">Orders this week</h1><p id="p">Supercharge your sales</p>`)
	if r := pass("done"); len(r.Passed) != 2 || r.Passed[1].Passes[0] != "08 Copy" {
		t.Fatalf("done: %+v", r.Passed)
	}
	hist, _ := srv.canvasFor(cv.ID).History(cv.ID)
	labels := 0
	for _, v := range hist.Versions {
		if strings.HasPrefix(v.Label, "Unslop · ") {
			labels++
		}
	}
	if labels != 2 {
		t.Fatalf("versions labelled: %d", labels)
	}
	story, err := srv.unslopStory(proto.UnslopStepsParams{ID: cv.ID, Frame: f})
	if err != nil || len(story.Docs) != 3 || len(story.Steps) != 2 || !strings.Contains(story.Docs[0], "padding:13px") {
		t.Fatalf("story: %d docs, %v", len(story.Docs), err)
	}
	// Leaving spacing out: the copy keeps the copy's words, its padding back.
	left, err := srv.leaveOut(proto.UnslopLeaveOutParams{ID: cv.ID, Frame: f, Step: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := srv.canvasFor(cv.ID).Get(cv.ID)
	var copy proto.CanvasFrame
	for _, fr := range got.Frames {
		if fr.ID == left.Frame {
			copy = fr
		}
	}
	if !strings.Contains(copy.HTML, "padding:13px") || !strings.Contains(copy.HTML, "Orders this week") || !strings.Contains(copy.Name, "without 03 spacing") {
		t.Fatalf("left out: %q %s", copy.Name, copy.HTML)
	}
}
