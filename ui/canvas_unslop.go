package ui

import (
	"fmt"
	"strings"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// Unslop on the Canvas page: one click reads the design shown for what
// makes it look AI-made, copies it, shows the copy, and has the canvas's
// agent redesign it pass by pass, with nothing more to ask.

func (a *App) unslopCanvas(frame string) {
	cv, cl := a.canvases.Canvas(), a.client
	if cv == nil || cl == nil || a.unslopping {
		return
	}
	a.unslopping = true
	id := cv.ID
	go func() {
		u, err := cl.CanvasUnslop(proto.CanvasUnslopParams{ID: id, Frame: frame, Start: true})
		a.onMain(func() {
			a.unslopping = false
			if err != nil {
				a.notify(strings.TrimPrefix(client.Explain(err), "canvas: "))
				return
			}
			o := a.canvasOptions()
			a.canvases.Goto(o, id, u.Frame)
			a.canvases.OpenAssistant()
			if cur := a.canvases.Canvas(); cur != nil && cur.ID == id {
				a.canvasTell(*cur, u.Ask)
			}
			a.notify(fmt.Sprintf("Unslop: %d/100, %d findings; the agent redesigns the copy", u.Score, len(u.Findings)))
		})
	}()
}
