package ui

import (
	"bytes"
	"image"
	_ "image/png" // the Browser's screenshots
	"net/url"
	"strings"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// A Browser's page put on a canvas: its screenshot as a design named
// Current, so a build compares its directions with the page as it runs.

// pageToCanvas puts png, the page at u width CSS pixels wide (0 unknown),
// on the canvas last open on the Canvas page, or a new one.
func (a *App) pageToCanvas(png []byte, u string, width int) {
	cl := a.client
	if cl == nil {
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
	if err != nil {
		a.onMain(func() { a.status = "The page's screenshot isn't a picture: " + err.Error() })
		return
	}
	w, h := cfg.Width, cfg.Height
	if width > 0 && width < w {
		h, w = h*width/w, width // a screen's pixels to the page's
	}
	var id string
	done := make(chan struct{})
	a.onMain(func() {
		if cv := a.canvases.Canvas(); cv != nil && cv.Kind == proto.CanvasMockup {
			id = cv.ID
		}
		close(done)
	})
	<-done
	title := "Pages"
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		title = p.Host + strings.TrimSuffix(p.Path, "/")
	}
	if id == "" {
		cv, err := cl.CreateCanvas(proto.CanvasCreateParams{Title: title, Kind: proto.CanvasMockup})
		if err != nil {
			a.onMain(func() { a.status = "Couldn't make a canvas: " + client.Explain(err) })
			return
		}
		id = cv.ID
	}
	name, err := cl.AddCanvasAsset(id, "page-"+time.Now().Format("20060102-150405")+".png", png)
	if err == nil {
		var cv proto.Canvas
		desc := "The page as it runs: " + u
		cv, err = cl.EditCanvas(proto.CanvasEdit{ID: id, Op: proto.CanvasOpFrameAdd, FrameInfo: &proto.CanvasFrame{Name: "Current", Width: w, Height: h, Description: desc}})
		if err == nil {
			f := cv.Frames[len(cv.Frames)-1]
			_, err = cl.EditCanvas(proto.CanvasEdit{ID: id, Op: proto.CanvasOpElementSet, Frame: f.ID,
				Element: &proto.CanvasElement{Type: proto.ShapeImage, X: 0, Y: 0, W: float64(w), H: float64(h), Src: name}})
		}
	}
	a.onMain(func() {
		if err != nil {
			a.status = "Couldn't put the page on a canvas: " + client.Explain(err)
			return
		}
		a.notify("The page is on the canvas as “Current”, to compare designs with")
	})
}
