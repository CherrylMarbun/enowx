package canvasview

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
)

// The stage shows the frame open, scaled to fit: its mockup in a web view,
// its shapes drawn over it, each one an element the user selects, moves,
// resizes and edits in Design mode.

// dragState is a shape being moved or resized: where it shows meanwhile.
type dragState struct {
	id         string
	resize     bool
	x, y, w, h float64
	moved      bool
}

// textEdit is a shape's text being edited in place.
type textEdit struct {
	id   string
	text string
}

// highlightFor is how long an agent's change stays outlined.
const highlightFor = 1600 * time.Millisecond

func color(s string, fallback ui.Color) ui.Color {
	if c, ok := canvas.ParseColor(s); ok {
		return ui.RGB(c.R, c.G, c.B)
	}
	return fallback
}

// stage draws the frame open.
func (v *View) stage(c *ui.Context, o Options) {
	f := v.frameOf()
	width, height := float64(f.Width), float64(f.Height)
	if v.preview > 0 && strings.TrimSpace(f.HTML) != "" {
		height = math.Max(height*float64(v.preview)/width, height)
		width = float64(v.preview)
	}
	area := ui.Scroll(c.Key("stage")).Grow(1).MinWidth(0).FillHeight().Background(theme.Sunken)
	b := area.Bounds()
	scale := float32(0.5)
	if b.W > 0 {
		scale = float32(math.Min(math.Min(float64(b.W-48)/width, float64(b.H-48)/height), 1))
		scale = max(scale, 0.1)
	}
	v.scale = scale
	s := float64(scale)
	now := time.Now()
	area.Children(func() {
		ui.Column(c).FillWidth().Padding(24).AlignItems(ui.Center).Children(func() {
			board := ui.Box(c.Key("board:"+f.ID)).Size(float32(width*s), float32(height*s)).Shrink(0).
				Background(color(f.Background, ui.RGB(255, 255, 255))).Radius(4).Clip().
				Shadow(0, 2, 12, 0, theme.Fade(theme.TextStrong, 0.12)).Label("Frame " + f.Name)
			if v.mode == modeDesign && board.Clicked() {
				v.sel, v.connect = "", ""
				if o.Web != nil {
					o.Web.outline("")
				}
			}
			board.Children(func() {
				if strings.TrimSpace(f.HTML) != "" {
					v.mockup(c, o, f, float32(width*s), float32(height*s), s)
				}
				v.arrows(c, f, s)
				for _, el := range f.Elements {
					if el.Type == proto.ShapeLine || el.Type == proto.ShapeArrow {
						v.arrowHandle(c.Key(el.ID), o, f, el, s)
						continue
					}
					v.shape(c.Key(el.ID), o, el, s, now)
				}
			})
		})
	})
	if v.lastAt.Add(highlightFor).After(now) {
		c.After(highlightFor - now.Sub(v.lastAt) + 10*time.Millisecond)
	}
}

// mockup is the frame's HTML: in a web view, or, where there is none,
// its elements listed as they come.
func (v *View) mockup(c *ui.Context, o Options, f proto.CanvasFrame, w, h float32, s float64) {
	if o.Web != nil {
		web := o.Web.view(c.Key("web"), w, h, s).Absolute().Left(0).Top(0)
		if v.mode == modeDesign {
			web.Opacity(1)
		}
		return
	}
	ui.Column(c.Key("outline")).Absolute().Left(0).Top(0).Size(w, h).Padding(16).Gap(4).Children(func() {
		ui.Text(c, "This device shows mockups in a web view: here, their elements.").FontSize(11).TextColor(ui.RGB(100, 116, 139))
		for _, el := range canvas.Outline(f.HTML) {
			row := ui.Row(c.Key(el.ID)).Padding(2, 6, 2, 6+float32(el.Depth)*12).Radius(4).Gap(6)
			if v.recent(el.ID) {
				row.Background(ui.RGBA(99, 102, 241, 0.18))
			}
			row.Children(func() {
				ui.Text(c, "#"+el.ID).FontSize(11).TextColor(ui.RGB(79, 70, 229))
				ui.Text(c, el.Text).FontSize(11).TextColor(ui.RGB(15, 23, 42)).SingleLine().Shrink(1)
			})
		}
	})
}

// recent says element id is the one an agent changed moments ago.
func (v *View) recent(id string) bool {
	return id != "" && v.last.Element == id && time.Since(v.lastAt) < highlightFor
}

// placed is where shape el shows: as it is, or as being dragged.
func (v *View) placed(el proto.CanvasElement) proto.CanvasElement {
	if d := v.drag; d != nil && d.id == el.ID {
		el.X, el.Y, el.W, el.H = d.x, d.y, d.w, d.h
	}
	return el
}

// shape draws a shape and handles the pointer on it.
func (v *View) shape(c *ui.Context, o Options, el proto.CanvasElement, s float64, now time.Time) {
	shown := v.placed(el)
	st := canvas.StyleOf(shown)
	box := ui.Box(c).Absolute().Left(float32(shown.X*s)).Top(float32(shown.Y*s)).Size(float32(shown.W*s), float32(shown.H*s)).
		Label(el.ID)
	fill, stroke := color(st.Fill, ui.Color{}), color(st.Stroke, ui.Color{})
	switch el.Type {
	case proto.ShapeEllipse, proto.ShapeDiamond:
		typ, width := el.Type, float32(st.StrokeWidth*s)
		box.Draw(func(p *ui.Painter, r ui.Rect) {
			var path ui.Path
			if typ == proto.ShapeEllipse {
				ellipsePath(&path, r)
			} else {
				path.MoveTo(r.X+r.W/2, r.Y).LineTo(r.X+r.W, r.Y+r.H/2).LineTo(r.X+r.W/2, r.Y+r.H).LineTo(r.X, r.Y+r.H/2).Close()
			}
			if st.Fill != "" {
				p.FillPath(&path, fill)
			}
			if st.Stroke != "" {
				p.StrokePath(&path, width, stroke)
			}
		})
	case proto.ShapeImage:
		box.Background(ui.RGB(226, 232, 240)).Radius(float32(el.Radius * s))
	default:
		if st.Fill != "" {
			box.Background(fill)
		}
		if st.Stroke != "" {
			box.Border(float32(math.Max(st.StrokeWidth*s, 1)), stroke)
		}
		box.Radius(float32(el.Radius * s))
		if el.Type == proto.ShapeSticky {
			box.Shadow(0, 1, 4, 0, ui.RGBA(15, 23, 42, 0.15))
		}
	}
	selected := v.sel == el.ID
	if v.recent(el.ID) {
		box.Shadow(0, 0, 0, 3, theme.Accent)
	} else if selected {
		box.Shadow(0, 0, 0, 2, ui.RGB(99, 102, 241))
	} else if v.connect != "" && box.Hovered() {
		box.Shadow(0, 0, 0, 2, ui.RGBA(99, 102, 241, 0.5))
	}
	if v.mode == modeDesign {
		box.Cursor(ui.CursorPointer)
		v.handlePointer(c, o, box, el, s)
	}
	box.Children(func() {
		switch {
		case v.editing != nil && v.editing.id == el.ID:
			in := ui.TextInput(c.Key("edit"), &v.editing.text).FillWidth().FillHeight().FontSize(float32(st.FontSize * s)).
				TextColor(color(st.Color, ui.RGB(15, 23, 42))).Padding(4).AutoFocus().Label("Text of " + el.ID)
			if in.Submitted() || in.Shortcut(0, ui.KeyEscape) || !in.Focused() && v.editing.text != el.Text {
				el.Text = v.editing.text
				v.editing = nil
				v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpElementSet, Element: &el})
			}
		case el.Type == proto.ShapeImage:
			ui.Column(c).FillWidth().FillHeight().Center().Gap(4).Children(func() {
				ui.Icon(c, icon.File).FontSize(float32(math.Max(14*s, 10))).TextColor(ui.RGB(100, 116, 139))
				ui.Text(c, el.Src).FontSize(float32(math.Max(11*s, 9))).TextColor(ui.RGB(100, 116, 139)).SingleLine()
			})
		case el.Text != "":
			shapeText(c, shown, st, s)
		}
		if selected && v.mode == modeDesign && v.editing == nil {
			v.resizeHandle(c, o, el, s)
		}
		if v.recent(el.ID) && v.last.Session != "" {
			who := "Agent"
			if o.AgentName != nil {
				who = o.AgentName(v.last.Session)
			}
			ui.Row(c.Key("who")).Absolute().Left(0).Top(-22).Height(18).PaddingX(6).Radius(4).Background(theme.Accent).Children(func() {
				ui.Text(c, who).FontSize(10.5).TextColor(ui.RGB(255, 255, 255)).SingleLine()
			})
		}
	})
	_ = now
}

func shapeText(c *ui.Context, el proto.CanvasElement, st canvas.Style, s float64) {
	align := ui.Center
	switch el.Align {
	case "left":
		align = ui.Start
	case "right":
		align = ui.End
	}
	ui.Column(c).FillWidth().FillHeight().Padding(float32(8 * s)).Justify(ui.Center).AlignItems(align).Children(func() {
		t := ui.Text(c, el.Text).FontSize(float32(st.FontSize * s)).TextColor(color(st.Color, ui.RGB(15, 23, 42))).TextAlign(align)
		if el.Bold {
			t.FontWeight(700)
		}
	})
}

func ellipsePath(p *ui.Path, r ui.Rect) {
	const k = 0.5523
	cx, cy, rx, ry := r.X+r.W/2, r.Y+r.H/2, r.W/2, r.H/2
	p.MoveTo(cx+rx, cy).
		CubeTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry).
		CubeTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy).
		CubeTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry).
		CubeTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy).Close()
}

// handlePointer selects shape el on a click, connects an arrow to it, edits
// its text on a double click, and moves it as it is dragged.
func (v *View) handlePointer(c *ui.Context, o Options, box ui.Element, el proto.CanvasElement, s float64) {
	if dx, dy, ok := box.Dragged(); ok {
		if v.drag == nil || v.drag.id != el.ID {
			v.drag = &dragState{id: el.ID, x: el.X, y: el.Y, w: el.W, h: el.H}
		}
		if (dx != 0 || dy != 0) && !v.drag.resize {
			v.drag.x += float64(dx) / s
			v.drag.y += float64(dy) / s
			v.drag.moved = true
		}
	} else if d := v.drag; d != nil && d.id == el.ID && !d.resize {
		v.drag = nil
		if d.moved {
			el.X, el.Y = math.Round(d.x), math.Round(d.y)
			v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpElementSet, Element: &el})
		}
	}
	if box.DoubleClicked() && el.Type != proto.ShapeImage {
		v.sel, v.editing = el.ID, &textEdit{id: el.ID, text: el.Text}
		return
	}
	if box.Clicked() {
		if v.connect != "" && v.connect != el.ID {
			arrow := proto.CanvasElement{Type: proto.ShapeArrow, From: v.connect, To: el.ID}
			v.connect = ""
			v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpElementSet, Element: &arrow})
			return
		}
		v.sel, v.panel = el.ID, "layers"
	}
}

// resizeHandle is the selected shape's corner, which drags its size.
func (v *View) resizeHandle(c *ui.Context, o Options, el proto.CanvasElement, s float64) {
	h := ui.Box(c.Key("resize")).Absolute().Right(-5).Bottom(-5).Size(10, 10).Radius(2).
		Background(ui.RGB(255, 255, 255)).Border(1.5, ui.RGB(99, 102, 241)).Cursor(ui.CursorResizeNWSE).Label("Resize " + el.ID)
	if dx, dy, ok := h.Dragged(); ok {
		if v.drag == nil || v.drag.id != el.ID || !v.drag.resize {
			v.drag = &dragState{id: el.ID, resize: true, x: el.X, y: el.Y, w: el.W, h: el.H}
		}
		v.drag.w = math.Max(v.drag.w+float64(dx)/s, 8)
		v.drag.h = math.Max(v.drag.h+float64(dy)/s, 8)
		v.drag.moved = v.drag.moved || dx != 0 || dy != 0
	} else if d := v.drag; d != nil && d.id == el.ID && d.resize {
		v.drag = nil
		if d.moved {
			el.W, el.H = math.Round(d.w), math.Round(d.h)
			v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpElementSet, Element: &el})
		}
	}
}

// arrows draws the frame's lines and arrows, following the shapes they
// join as those move.
func (v *View) arrows(c *ui.Context, f proto.CanvasFrame, s float64) {
	shown := f
	shown.Elements = make([]proto.CanvasElement, len(f.Elements))
	for i, el := range f.Elements {
		shown.Elements[i] = v.placed(el)
	}
	ui.Box(c.Key("arrows")).Absolute().Left(0).Top(0).FillWidth().FillHeight().Draw(func(p *ui.Painter, r ui.Rect) {
		for _, el := range shown.Elements {
			if el.Type != proto.ShapeLine && el.Type != proto.ShapeArrow {
				continue
			}
			st := canvas.StyleOf(el)
			col := color(st.Stroke, ui.RGB(71, 85, 105))
			if v.sel == el.ID || v.recent(el.ID) {
				col = ui.RGB(99, 102, 241)
			}
			x1, y1, x2, y2 := canvas.Ends(shown, el)
			w := float32(math.Max(st.StrokeWidth*s, 1))
			p.Line(r.X+float32(x1*s), r.Y+float32(y1*s), r.X+float32(x2*s), r.Y+float32(y2*s), w, col)
			if el.Type == proto.ShapeArrow {
				ax, ay, bx, by := canvas.ArrowHead(x1, y1, x2, y2, 8+st.StrokeWidth*2)
				var head ui.Path
				head.MoveTo(r.X+float32(x2*s), r.Y+float32(y2*s)).LineTo(r.X+float32(ax*s), r.Y+float32(ay*s)).
					LineTo(r.X+float32(bx*s), r.Y+float32(by*s)).Close()
				p.FillPath(&head, col)
			}
		}
	})
}

// arrowHandle is an arrow's label at its middle, which selects it.
func (v *View) arrowHandle(c *ui.Context, o Options, f proto.CanvasFrame, el proto.CanvasElement, s float64) {
	shown := f
	shown.Elements = make([]proto.CanvasElement, len(f.Elements))
	for i, x := range f.Elements {
		shown.Elements[i] = v.placed(x)
	}
	x1, y1, x2, y2 := canvas.Ends(shown, el)
	mx, my := (x1+x2)/2*s, (y1+y2)/2*s
	st := canvas.StyleOf(el)
	label := ui.Row(c).Absolute().Left(float32(mx) - 60).Top(float32(my) - 11).Width(120).Height(22).Justify(ui.Center).AlignItems(ui.Center).Label(el.ID)
	if v.mode == modeDesign {
		label.Cursor(ui.CursorPointer)
		if label.Clicked() {
			v.sel, v.panel = el.ID, "layers"
		}
	}
	label.Children(func() {
		if el.Text == "" {
			return
		}
		ui.Text(c, el.Text).FontSize(float32(st.FontSize * s * 0.85)).TextColor(color(st.Color, ui.RGB(15, 23, 42))).
			Background(ui.RGBA(255, 255, 255, 0.9)).PaddingX(4).Radius(3).SingleLine()
	})
	_ = o
}
