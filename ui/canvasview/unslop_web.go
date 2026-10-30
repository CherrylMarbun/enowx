package canvasview

import (
	"strconv"
	"time"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/unslopview"
)

// A redesign's reads in the design's page, as they come (unslopview's
// scripts): the marks of each read, the regions agents redesign, and
// each change an agent makes tagged with its pass.

// scan draws run's latest read over the page, once per read of the design
// shown.
func (w *Web) scan(run *proto.CanvasUnslopRun, frame string) {
	if run == nil || run.Frame != frame || w.page == nil {
		w.scanned = ""
		return
	}
	key := run.Frame + "/" + strconv.Itoa(run.Scans) + "/" + w.shown
	if w.scanned == key {
		return
	}
	w.scanned = key
	page, code := w.page, unslopview.ScanScript+"("+unslopview.ScanItems(run)+")"
	go func() {
		// A page reloaded just now takes a moment to hold the design.
		time.Sleep(350 * time.Millisecond)
		_, _ = page.Eval(code)
	}()
}

// tagPass tags element id, changed by session, with the pass under way:
// its region's, in its color, for a region's agent.
func (w *Web) tagPass(id, session string) {
	label, color := w.pass, ""
	if t, ok := w.regionTags[session]; ok {
		label, color = t.Label, t.Color
	}
	if w.page == nil || label == "" || id == "" {
		return
	}
	page, code := w.page, unslopview.PassTagScript+"("+js(id)+","+js(label)+","+js(color)+")"
	go func() { _, _ = page.Eval(code) }()
}

// outlineRegions outlines run's regions in the page, when they changed.
func (w *Web) outlineRegions(run *proto.CanvasUnslopRun, frame string, agentName func(string) string) {
	if run != nil && run.Frame != frame {
		run = nil
	}
	data, tags := unslopview.RegionMarks(run, agentName)
	w.regionTags = tags
	key := data + "/" + w.shown + "/" + w.scanned
	if w.page == nil || w.outlined == key {
		return
	}
	w.outlined = key
	page := w.page
	go func() {
		time.Sleep(350 * time.Millisecond)
		_, _ = page.Eval(unslopview.RegionsScript + "(" + data + ")")
	}()
}
