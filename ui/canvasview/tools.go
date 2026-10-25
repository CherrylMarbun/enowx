package canvasview

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// toolbar is the frames, the mode, the preview's width, and the shapes to
// add.
func (v *View) toolbar(c *ui.Context, o Options, cv proto.Canvas) {
	f := v.frameOf()
	ui.Row(c).FillWidth().Height(40).Shrink(0).PaddingX(12).Gap(6).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		for _, fr := range cv.Frames {
			on := fr.ID == v.frame
			tab := ui.ButtonBase(c.Key(fr.ID)).Height(26).PaddingX(10).Radius(6).Label("Frame tab " + fr.Name).Cursor(ui.CursorPointer)
			if on {
				tab.Background(theme.Selected)
			} else if tab.Hovered() {
				tab.Background(theme.Hover)
			}
			tab.Children(func() {
				col := theme.Muted
				if on {
					col = theme.Accent
				}
				ui.Text(c, fmt.Sprintf("%s  %d×%d", fr.Name, fr.Width, fr.Height)).FontSize(11.5).TextColor(col).SingleLine()
			})
			if tab.Clicked() {
				v.frame, v.sel, v.preview = fr.ID, "", 0
			}
		}
		add := widget.IconButton(c, icon.Plus, "Add a frame", false)
		v.menu.OpenBelow(add, "add-frame", 28)
		v.menu.Show(c, add, "add-frame", func(done func()) {
			for _, d := range canvas.Devices {
				if widget.MenuItem(c.Key(d.Name), icon.Square, fmt.Sprintf("%s · %d×%d", d.Name, d.Width, d.Height), false) {
					v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpFrameAdd, FrameInfo: &proto.CanvasFrame{Name: d.Name}})
					done()
				}
			}
			if len(cv.Frames) > 1 {
				widget.MenuSeparator(c)
				if widget.MenuItem(c.Key("rm"), icon.Trash, "Remove "+f.Name, true) {
					v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpFrameRemove, Frame: f.ID})
					done()
				}
			}
		})
		if cv.Kind == proto.CanvasMockup || strings.TrimSpace(f.HTML) != "" {
			ins := widget.Button(c, widget.Secondary, icon.Plus, "Insert", "").Height(26).PaddingX(10).Tooltip("Insert a component into the mockup")
			v.menu.OpenBelow(ins, "insert", 26)
			v.menu.Show(c, ins, "insert", func(done func()) {
				for _, comp := range canvas.Components {
					if widget.MenuItem(c.Key(comp.Name), icon.Square, comp.Name, false) {
						id, html := canvas.ComponentHTML(comp, f.HTML)
						v.sel = id
						v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpHTMLPatch, Target: id, HTML: html})
						done()
					}
				}
			})
		}
		ui.Spacer(c)
		if strings.TrimSpace(f.HTML) != "" {
			widths := []int{0, 1440, 768, 390}
			names := []string{"Frame", "Desktop", "Tablet", "Mobile"}
			cur := 0
			for i, w := range widths {
				if w == v.preview {
					cur = i
				}
			}
			if i := widget.Segmented(c.Key("preview"), names, cur, nil); i >= 0 {
				v.preview = widths[i]
			}
			if i := widget.Segmented(c.Key("mode"), []string{"Design", "Interact"}, v.mode, nil); i >= 0 {
				v.mode = i
			}
		}
	})
}

// palette is the shapes to add, and Connect, down the stage's side in
// Design mode.
func (v *View) palette(c *ui.Context, o Options) {
	if v.mode != modeDesign {
		return
	}
	ui.Column(c.Key("palette")).Width(44).Shrink(0).FillHeight().Padding(8, 0).Gap(4).AlignItems(ui.Center).
		Background(theme.Panel).BorderWidth(0, 1, 0, 0).BorderColor(theme.Border).Children(func() {
		for _, t := range []struct {
			typ   string
			svg   *ui.SVG
			label string
		}{{proto.ShapeRect, icon.Square, "Rectangle"}, {proto.ShapeEllipse, icon.Circle, "Ellipse"}, {proto.ShapeDiamond, icon.Diamond, "Decision"},
			{proto.ShapeText, icon.Type, "Text"}, {proto.ShapeSticky, icon.Sticky, "Sticky note"}} {
			if widget.IconButton(c.Key(t.typ), t.svg, "Add "+strings.ToLower(t.label), false).Clicked() {
				v.addShape(o, t.typ)
			}
		}
		ui.Box(c).Size(20, 1).Background(theme.Border)
		label := "Connect: select a shape, then the one to point to"
		on := v.connect != ""
		if on {
			label = "Click the shape to point to"
		}
		if b := widget.IconButton(c.Key("connect"), icon.ArrowLink, label, on); b.Clicked() {
			switch {
			case on:
				v.connect = ""
			case v.sel != "":
				v.connect = v.sel
			}
		}
	})
}

// addShape adds a shape of typ in the frame's middle, selected.
func (v *View) addShape(o Options, typ string) {
	f := v.frameOf()
	el := proto.CanvasElement{Type: typ, W: 180, H: 90, X: float64(f.Width)/2 - 90, Y: float64(f.Height)/2 - 45}
	switch typ {
	case proto.ShapeText:
		el.Text, el.H = "Text", 40
	case proto.ShapeSticky:
		el.Text, el.W, el.H = "Note", 160, 160
	case proto.ShapeRect:
		el.Radius = 8
	}
	for n := len(f.Elements) + 1; ; n++ {
		id := fmt.Sprintf("%s-%d", typ, n)
		if _, taken := shapeOf(f, id); !taken {
			el.ID = id
			break
		}
	}
	v.sel, v.panel = el.ID, "layers"
	v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpElementSet, Element: &el})
}

func shapeOf(f proto.CanvasFrame, id string) (proto.CanvasElement, bool) {
	for _, el := range f.Elements {
		if el.ID == id {
			return el, true
		}
	}
	return proto.CanvasElement{}, false
}

// keys are the page's shortcuts: undo, redo, delete the selection.
func (v *View) keys(c *ui.Context, o Options) {
	if c.Shortcut(ui.Cmd, ui.KeyZ) {
		v.undo(o, false)
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) {
		v.undo(o, true)
	}
	if v.editing == nil && v.sel != "" && v.mode == modeDesign && (c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace)) {
		v.remove(o, v.sel)
	}
	if v.connect != "" && c.Shortcut(0, ui.KeyEscape) {
		v.connect = ""
	}
}

// remove removes element id: a shape, or an element of the mockup.
func (v *View) remove(o Options, id string) {
	if _, ok := shapeOf(v.frameOf(), id); ok {
		v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpElementDel, Target: id})
	} else {
		v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpHTMLRemove, Target: id})
	}
	v.sel = ""
}
