package canvasview

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Picked is the designs picked to compare (frame IDs).
func (v *View) Picked() []string {
	if v.canvas == nil {
		return nil
	}
	var out []string
	for _, f := range v.comparing(*v.canvas) {
		out = append(out, f.ID)
	}
	return out
}

// comparing is the designs picked to compare that the canvas still has, in
// its order.
func (v *View) comparing(cv proto.Canvas) []proto.CanvasFrame {
	var out []proto.CanvasFrame
	for _, f := range cv.Frames {
		if v.cmpSel[f.ID] {
			out = append(out, f)
		}
	}
	return out
}

// grid is every design at once, to look over, open, star and pick to
// compare.
func (v *View) grid(c *ui.Context, o Options, cv proto.Canvas) {
	ui.Scroll(c.Key("grid")).Grow(1).MinWidth(0).FillHeight().Background(theme.Sunken).Children(func() {
		ui.Column(c).FillWidth().Padding(20).Gap(12).Children(func() {
			if n := len(v.comparing(cv)); n > 0 {
				ui.Text(c, fmt.Sprintf("%d picked to compare (2 to 4): Compare above shows them side by side", n)).FontSize(11.5).TextColor(theme.Muted)
			}
			ui.Row(c).FillWidth().Gap(16).Wrap().Children(func() {
				for i, f := range cv.Frames {
					v.gridCard(c.Key(f.ID), o, cv, i, f)
				}
			})
		})
	})
}

func (v *View) gridCard(c *ui.Context, o Options, cv proto.Canvas, i int, f proto.CanvasFrame) {
	card := ui.Column(c).Width(300).Padding(10).Gap(8).Radius(10).Background(theme.Raised).Border(1, theme.Border)
	if v.working(f) {
		card.Border(1.5, theme.Accent)
	}
	card.Children(func() {
		pv := ui.ButtonBase(c.Key("open")).Cursor(ui.CursorPointer).Label(fmt.Sprintf("Open design %d", i+1))
		pv.Children(func() { v.designPreview(c.Key("pv"), o, f, 278) })
		if pv.Clicked() {
			v.frame, v.sel, v.view = f.ID, "", viewDesign
		}
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("%d · %s", i+1, f.Name)).FontSize(12.5).FontWeight(600).SingleLine().Shrink(1).Grow(1).MinWidth(0)
			if cv.Winner == f.ID {
				widget.Tag(c, "Winner", theme.Done)
			}
			v.starButton(c, o, f)
		})
		if g := groupOf(f); g != "" || f.Description != "" {
			ui.Text(c, strings.Trim(g+" — "+f.Description, " —")).FontSize(11).TextColor(theme.Muted)
		}
		on := v.cmpSel[f.ID]
		label, svg := "Pick to compare", icon.Layers
		if on {
			label, svg = "Picked to compare", icon.Check
		}
		b := widget.Button(c.Key("cmp"), widget.Secondary, svg, label, "").Height(26).PaddingX(10).Label(fmt.Sprintf("Compare design %d", i+1))
		if on {
			b.Border(1, theme.Accent)
		}
		if b.Clicked() {
			if v.cmpSel == nil {
				v.cmpSel = map[string]bool{}
			}
			if !on && len(v.comparing(cv)) >= 4 {
				v.err = "Compare up to 4 designs"
			} else {
				v.cmpSel[f.ID] = !on
			}
		}
	})
}

// compareView is the designs picked, side by side at one scale, scrolling
// together.
func (v *View) compareView(c *ui.Context, o Options, cv proto.Canvas) {
	list := v.comparing(cv)
	if len(list) < 2 {
		v.view = viewGrid
		return
	}
	area := ui.Scroll(c.Key("compare")).Grow(1).MinWidth(0).FillHeight().Background(theme.Sunken)
	b := area.Bounds()
	widest := 1
	for _, f := range list {
		widest = max(widest, f.Width)
	}
	w := float32(360)
	if b.W > 0 {
		w = float32(math.Max(float64(b.W-40-float32(len(list)-1)*16)/float64(len(list)), 160))
	}
	area.Children(func() {
		ui.Row(c).Padding(20).Gap(16).AlignItems(ui.Start).Children(func() {
			for _, f := range list {
				i := slices.IndexFunc(cv.Frames, func(x proto.CanvasFrame) bool { return x.ID == f.ID })
				// One scale for all: the widest design fits its column.
				fw := w * float32(f.Width) / float32(widest)
				ui.Column(c.Key(f.ID)).Width(w).Gap(8).Children(func() {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, fmt.Sprintf("%d · %s", i+1, f.Name)).FontSize(12.5).FontWeight(600).SingleLine().Grow(1).Shrink(1).MinWidth(0)
						v.starButton(c, o, f)
						if widget.IconButton(c.Key("pick"), icon.Award, "Pick "+f.Name+" as the winner", cv.Winner == f.ID).Clicked() {
							v.edit(o, proto.CanvasEdit{Op: proto.CanvasOpPick, Frame: f.ID})
						}
					})
					open := ui.ButtonBase(c.Key("open")).Cursor(ui.CursorPointer).Label(fmt.Sprintf("Open design %d", i+1))
					open.Children(func() { v.designPreview(c.Key("pv"), o, f, fw) })
					if open.Clicked() {
						v.frame, v.view = f.ID, viewDesign
					}
					if f.Description != "" {
						ui.Text(c, f.Description).FontSize(11).TextColor(theme.Muted)
					}
				})
			}
		})
	})
}
