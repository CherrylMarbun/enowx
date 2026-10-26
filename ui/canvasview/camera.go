package canvasview

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The stage is an endless board, as a design tool's: the design shows at
// its own size times the zoom, wherever the user moved it. Scrolling moves
// it (two fingers, or a wheel; Shift for sideways), pinching or scrolling
// with Command (Control) zooms at the pointer, and dragging the board
// around the design, or with the middle button anywhere, moves it too.

// Zoom's bounds.
const (
	minZoom = 0.05
	maxZoom = 4
)

// camera is where the design shows on the stage: its top left corner at
// x, y, zoom z.
type camera struct {
	x, y, z float32
	key     string // the design and width it was placed for
	panning bool   // a drag moving the board
	moved   bool   // it moved since the press
	px, py  float32
}

// zoomAt zooms by factor, keeping the point under x, y where it is.
func (cam *camera) zoomAt(x, y, factor float32) {
	z := min(max(cam.z*factor, minZoom), maxZoom)
	f := z / cam.z
	cam.x = x - (x-cam.x)*f
	cam.y = y - (y-cam.y)*f
	cam.z = z
}

// fitWidth shows the design's whole width (never larger than its own
// size), from its top, in a stage aw wide.
func (cam *camera) fitWidth(aw, width float32) {
	cam.z = min(max((aw-96)/width, minZoom), 1)
	cam.x = (aw - width*cam.z) / 2
	cam.y = 40
}

// fitAll shows the whole design in a stage aw×ah.
func (cam *camera) fitAll(aw, ah, width, height float32) {
	cam.z = min(max(min((aw-96)/width, (ah-96)/height), minZoom), 1)
	cam.x = (aw - width*cam.z) / 2
	cam.y = (ah - height*cam.z) / 2
}

// cameraInput moves and zooms the board as the user scrolls, pinches and
// drags over an element whose box starts at ox, oy on the stage. A press
// inside design (stage coordinates) is left to the design's own elements;
// click, when set, is what a press released without moving does.
func (v *View) cameraInput(ox, oy float32, design *ui.Rect, click func()) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		x, y := ev.X+ox, ev.Y+oy
		cam := &v.cam
		switch ev.Kind {
		case ui.InputZoom:
			cam.zoomAt(x, y, ev.Scale)
			return true
		case ui.InputScroll:
			if ev.Mods&(ui.Cmd|ui.Ctrl) != 0 {
				cam.zoomAt(x, y, float32(math.Exp(-float64(ev.DY)/120)))
				return true
			}
			dx, dy := ev.DX, ev.DY
			if ev.Mods&ui.Shift != 0 && dx == 0 {
				dx, dy = dy, 0
			}
			cam.x -= dx
			cam.y -= dy
			return true
		case ui.InputPointerDown:
			inDesign := design != nil && x >= design.X && y >= design.Y && x < design.X+design.W && y < design.Y+design.H
			if ev.Button == 2 || ev.Button == 0 && !inDesign {
				cam.panning, cam.moved, cam.px, cam.py = true, false, x, y
				return true
			}
		case ui.InputPointerMove:
			if cam.panning {
				if !cam.moved && math.Hypot(float64(x-cam.px), float64(y-cam.py)) < 3 {
					return true // a click's tremble
				}
				cam.x += x - cam.px
				cam.y += y - cam.py
				cam.px, cam.py, cam.moved = x, y, true
				return true
			}
		case ui.InputPointerUp:
			if cam.panning {
				cam.panning = false
				if !cam.moved && click != nil {
					click()
				}
				return true
			}
		}
		return false
	}
}

// dots draws the board's dots, moving and scaling with it.
func (v *View) dots(c *ui.Context) {
	cam := v.cam
	ui.Box(c.Key("dots")).Absolute().Left(0).Top(0).FillWidth().FillHeight().PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		step := float32(24) * cam.z
		for step < 12 {
			step *= 2
		}
		dot := theme.Fade(theme.Muted, 0.25)
		x0 := r.X + float32(math.Mod(float64(cam.x), float64(step)))
		y0 := r.Y + float32(math.Mod(float64(cam.y), float64(step)))
		for y := y0; y < r.Y+r.H; y += step {
			for x := x0; x < r.X+r.W; x += step {
				p.Fill(ui.Rect{X: x, Y: y, W: 1.5, H: 1.5}, dot, 0.75)
			}
		}
	})
}

// zoomBar is the zoom's controls, at the stage's bottom right: out, the
// zoom (a click shows the design at its own size), in, and fit.
func (v *View) zoomBar(c *ui.Context, aw, ah, width, height float32) {
	ui.Row(c.Key("zoom")).Absolute().Right(12).Bottom(12).Height(32).PaddingX(4).Gap(2).AlignItems(ui.Center).
		Radius(8).Background(theme.Panel).Border(1, theme.Border).Children(func() {
		if widget.IconButton(c.Key("out"), icon.Minus, "Zoom out", false).Clicked() {
			v.cam.zoomAt(aw/2, ah/2, 1/1.25)
		}
		pct := widget.Button(c.Key("pct"), widget.Secondary, nil, fmt.Sprintf("%d%%", int(math.Round(float64(v.cam.z*100)))), "").
			Height(26).PaddingX(8).Tooltip("Show it at its own size").Label("Show it at its own size")
		if pct.Clicked() {
			v.cam.zoomAt(aw/2, ah/2, 1/v.cam.z)
		}
		if widget.IconButton(c.Key("in"), icon.Plus, "Zoom in", false).Clicked() {
			v.cam.zoomAt(aw/2, ah/2, 1.25)
		}
		if widget.IconButton(c.Key("fit-width"), icon.FitWidth, "Fit its width", false).Clicked() {
			v.cam.fitWidth(aw, width)
		}
		if widget.IconButton(c.Key("fit"), icon.Maximize, "Fit it all", false).Clicked() {
			v.cam.fitAll(aw, ah, width, height)
		}
	})
}
