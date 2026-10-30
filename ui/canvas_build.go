package ui

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// The UI builder on the Canvas page: the Build dialog starts a build, the
// canvas's agent told what to do; its run comes as events; Compare's Pick
// sends the design chosen, with what to keep and change, to the agent.

// startBuild starts build p on the canvas shown, image (a screenshot to
// recreate) kept on the canvas first.
func (a *App) startBuild(p proto.CanvasBuildParams, image []byte) {
	cv, cl := a.canvases.Canvas(), a.client
	if cv == nil || cl == nil || a.buildStarting {
		return
	}
	a.buildStarting, a.buildErr = true, ""
	p.ID = cv.ID
	id := cv.ID
	go func() {
		var b proto.CanvasBuild
		var err error
		if len(image) > 0 {
			p.Image, err = cl.AddCanvasAsset(id, "screenshot-"+time.Now().Format("20060102-150405")+".png", image)
		}
		if err == nil {
			b, err = cl.CanvasBuild(p)
		}
		a.onMain(func() {
			a.buildStarting = false
			if err != nil {
				a.buildErr = strings.TrimPrefix(client.Explain(err), "canvas: ")
				return
			}
			if b.Run != nil {
				a.setBuildRun(*b.Run)
			}
			a.canvases.OpenAssistant()
			if cur := a.canvases.Canvas(); cur != nil && cur.ID == id {
				a.canvasTell(*cur, b.Ask)
			}
			a.notify("Build started: the agent writes the brief first")
		})
	}()
}

// buildEvent keeps a build's run as the daemon tells it.
func (a *App) buildEvent(data json.RawMessage) func() {
	var run proto.CanvasBuildRun
	if json.Unmarshal(data, &run) != nil {
		return nil
	}
	return func() { a.setBuildRun(run) }
}

func (a *App) setBuildRun(run proto.CanvasBuildRun) {
	if a.buildRuns == nil {
		a.buildRuns = map[string]proto.CanvasBuildRun{}
	}
	a.buildRuns[run.ID] = run
}

// buildRunOf is the build of the canvas shown, nil when none.
func (a *App) buildRunOf() *proto.CanvasBuildRun {
	cv := a.canvases.Canvas()
	if cv == nil {
		return nil
	}
	if run, ok := a.buildRuns[cv.ID]; ok {
		return &run
	}
	return nil
}

// pickDirection tells the build's agent the design picked, with what to
// keep and change.
func (a *App) pickDirection(cv proto.Canvas, frame, keep, change string) {
	n := slices.IndexFunc(cv.Frames, func(f proto.CanvasFrame) bool { return f.ID == frame })
	if n < 0 {
		return
	}
	text := fmt.Sprintf("I pick design %d (“%s”).", n+1, cv.Frames[n].Name)
	if keep != "" {
		text += " Keep: " + keep + "."
	}
	if change != "" {
		text += " Change: " + change + "."
	}
	text += " Refine it around that (canvas_build stage refining), keeping the other designs as they are; then check and finish it."
	a.canvasTell(cv, text)
	if cl := a.client; cl != nil {
		go func() { _, _ = cl.CanvasBuild(proto.CanvasBuildParams{ID: cv.ID, Stage: proto.BuildRefining}) }()
	}
	a.canvases.OpenAssistant()
}
