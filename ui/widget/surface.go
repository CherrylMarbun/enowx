package widget

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Card is a raised surface for a group of facts.
func Card(c *ui.Context) ui.Element {
	return Raise(ui.Column(c).MinWidth(0).Gap(10).Padding(14).Radius(theme.RadiusCard).
		Background(theme.Raised).Border(1, theme.Border))
}

// Segment is one part of a Meter.
type Segment struct {
	Fraction float64 // of the whole track, 0 to 1
	Color    ui.Color
}

// Meter is a thin track filled with segments, in order.
func Meter(c *ui.Context, parts ...Segment) {
	rest := 1.0
	ui.Row(c).FillWidth().Height(4).Radius(2).Clip().Background(theme.Selected).Children(func() {
		for i, p := range parts {
			f := min(max(p.Fraction, 0), rest)
			rest -= f
			if f > 0 {
				ui.Box(c.Key(i)).Height(4).Grow(float32(f)).Background(p.Color)
			}
		}
		if rest > 0 {
			ui.Box(c.Key("rest")).Height(4).Grow(float32(rest))
		}
	})
}

// Legend is a meter's key: a dot and what it stands for.
func Legend(c *ui.Context, col ui.Color, label string) {
	ui.Row(c).Gap(6).Shrink(0).Children(func() {
		Dot(c, col, false, 6)
		ui.Text(c, label).FontSize(11).TextColor(theme.Muted).SingleLine()
	})
}

// Level colors a fill by how full it is.
func Level(fill float64) ui.Color {
	switch {
	case fill > 0.9:
		return theme.Error
	case fill > 0.75:
		return theme.Warning
	}
	return theme.Accent
}

// OptionCard is one of a group of exclusive choices with a title and a
// line about it; it reports whether it was clicked.
func OptionCard(c *ui.Context, on bool, title, body string) bool {
	card := ui.ButtonBase(c).MinWidth(0).Padding(11, 12).Gap(10).Radius(theme.RadiusControl + 2).
		AlignItems(ui.Start).Cursor(ui.CursorPointer).Label(title)
	bg, border := theme.Fade(theme.AccentSoft, 0), theme.BorderStrong
	switch {
	case on:
		bg, border = theme.AccentSoft, theme.AccentLine
	case card.Hovered():
		border = theme.Fade(theme.Accent, 0.25)
	}
	card.Background(tint(card, "bg", bg)).Border(1, tint(card, "border", border))
	card.Children(func() {
		ring := ui.Box(c).Size(14, 14).Radius(7).Center().Margin(1, 0, 0, 0).Shrink(0)
		if on {
			ring.Border(1.2, theme.Accent).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(theme.Accent)
			})
		} else {
			ring.Border(1.2, theme.Idle)
		}
		ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(12.5)
			if body != "" {
				ui.Text(c, body).FontSize(11.5).TextColor(theme.Muted)
			}
		})
	})
	return card.Clicked()
}

// Notice is a quiet box that tells something about the form it sits in;
// col colors its edge, theme.BorderStrong when zero.
func Notice(c *ui.Context, svg *ui.SVG, col ui.Color, title, body string, extra func()) {
	border := theme.Border
	if col.A != 0 {
		border = theme.Fade(col, 0.3)
	} else {
		col = theme.TextSoft
	}
	ui.Row(c).FillWidth().Gap(12).Padding(12, 14).Radius(theme.RadiusCard).Background(theme.Raised).
		Border(1, border).AlignItems(ui.Start).Children(func() {
		if svg != nil {
			ui.Icon(c, svg).FontSize(16).TextColor(col).Margin(1, 0, 0, 0)
		}
		ui.Column(c).Gap(5).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(12.5)
			if body != "" {
				ui.Text(c, body).FontSize(11.5).TextColor(theme.Muted)
			}
			if extra != nil {
				extra()
			}
		})
	})
}

// StateCard tells what state a host is in: a badge colored by it, a title
// and message, then what can be done.
func StateCard(c *ui.Context, col ui.Color, svg *ui.SVG, title, msg string, badge, actions func()) {
	Float(ui.Column(c).Width(460).Padding(20).Gap(16).Radius(theme.RadiusPanel).
		Background(theme.Raised).Border(1, theme.BorderStrong)).Children(func() {
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(38, 38).Shrink(0).Radius(11).Background(theme.Selected).Border(1, theme.Border).
				Center().TextColor(col).Children(func() {
				if badge != nil {
					badge()
				} else {
					ui.Icon(c, svg).FontSize(18)
				}
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, title).FontSize(14)
				ui.Text(c, msg).FontSize(12).TextColor(theme.Muted)
			})
		})
		if actions != nil {
			ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(actions)
		}
	})
}

// EmptyState fills a view that has nothing yet: an icon, what is missing,
// and the one action that fixes it.
func EmptyState(c *ui.Context, svg *ui.SVG, title, body string, action func()) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
		Raise(ui.Box(c).Size(48, 48).Radius(14).Background(theme.Raised).Border(1, theme.Border).
			Center().TextColor(theme.Accent)).Children(func() {
			ui.Icon(c, svg).FontSize(20)
		})
		ui.Text(c, title).FontSize(15).Margin(4, 0, 0, 0)
		ui.Text(c, body).TextColor(theme.Muted).MaxWidth(420).TextAlign(ui.Center)
		if action != nil {
			ui.Box(c).Height(6)
			action()
		}
	})
}

// Spinner says something is under way: MyGo's turning spokes, in the
// accent, as high as size.
func Spinner(c *ui.Context, size float32) {
	ui.Spinner(c).FontSize(size).TextColor(theme.Accent).Shrink(0)
}
