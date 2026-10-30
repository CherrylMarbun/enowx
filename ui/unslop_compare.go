package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/unslopview"
)

// An unslop's before and after, compared: a Browser run's screenshots at
// each size, or a canvas's design and its unslopped copy, rendered.

// unslopCompare is the comparison open, its pictures as they come in.
type unslopCompare struct {
	view     unslopview.Compare
	shots    []unslopview.Shot
	loading  bool
	err      string
	from, to int
}

// compareBrowserUnslop compares session's page before and after its run.
func (a *App) compareBrowserUnslop(session string, run proto.CanvasUnslopRun) {
	cl := a.client
	if cl == nil {
		return
	}
	cmp := &unslopCompare{loading: true, from: run.From, to: run.Score}
	a.unslopComparing = cmp
	go func() {
		got, err := cl.BrowserUnslopShots(session)
		var shots []unslopview.Shot
		for _, s := range got.Shots {
			before, berr := ui.DecodeBitmap(s.Before)
			if berr != nil {
				continue
			}
			shot := unslopview.Shot{Label: s.Label, Before: before}
			if len(s.After) > 0 {
				shot.After, _ = ui.DecodeBitmap(s.After)
			}
			shots = append(shots, shot)
		}
		a.onMain(func() {
			cmp.loading, cmp.shots = false, shots
			switch {
			case err != nil:
				cmp.err = strings.TrimPrefix(client.Explain(err), "browser: ")
			case len(shots) == 0:
				cmp.err = "No pictures of this unslop were taken."
			}
		})
	}()
}

// compareCanvasUnslop compares canvas id's design run.Original with its
// unslopped copy run.Frame.
func (a *App) compareCanvasUnslop(id string, run proto.CanvasUnslopRun) {
	cl := a.client
	if cl == nil || run.Original == "" {
		return
	}
	cmp := &unslopCompare{loading: true, from: run.From, to: run.Score}
	a.unslopComparing = cmp
	render := func(frame string) (*ui.Bitmap, error) {
		out, err := cl.RenderCanvas(proto.CanvasRenderParams{ID: id, Frame: frame, Format: "png"})
		if err != nil {
			return nil, err
		}
		return ui.DecodeBitmap(out.Data)
	}
	go func() {
		before, err := render(run.Original)
		var after *ui.Bitmap
		if err == nil {
			after, err = render(run.Frame)
		}
		a.onMain(func() {
			cmp.loading = false
			if err != nil {
				cmp.err = strings.TrimPrefix(client.Explain(err), "canvas: ")
				return
			}
			cmp.shots = []unslopview.Shot{{Label: "Design", Before: before, After: after}}
		})
	}()
}

func (a *App) unslopCompareDialog(c *ui.Context) {
	cmp := a.unslopComparing
	if cmp == nil {
		return
	}
	cmp.view.Draw(c, unslopview.CompareOptions{Shots: cmp.shots, Loading: cmp.loading, Err: cmp.err,
		From: cmp.from, To: cmp.to, Close: func() { a.unslopComparing = nil }})
}
