// Package widget holds the controls every view shares, so one look and
// behavior is defined once: buttons, inputs, status marks, lists, cards,
// menus and dialogs. They draw with the tokens of package theme, so they
// follow the palette the user picks.
package widget

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// ButtonKind is the role of a button.
type ButtonKind int

const (
	Primary   ButtonKind = iota // the one action a view is for
	Secondary                   // the others
	Danger                      // destructive ones
)

// Button is a text button with an optional icon and shortcut hint.
func Button(c *ui.Context, kind ButtonKind, svg *ui.SVG, label, hint string) ui.Element {
	b := ui.ButtonBase(c).Gap(7).PaddingX(14).Height(theme.ControlHeight).Radius(theme.RadiusControl).
		Cursor(ui.CursorPointer).Label(label).Shrink(0)
	hover := b.Hovered() && !b.IsDisabled()
	switch kind {
	case Primary:
		top, bottom := theme.AccentHigh, theme.Accent
		if b.Pressed() {
			top = theme.Accent
		}
		b.Gradient(tint(b, "top", top), tint(b, "bottom", bottom), 180).TextColor(theme.OnAccent).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.3))
		if hover {
			b.Shadow(0, 8, 22, 0, theme.Fade(theme.Accent, 0.18))
		} else {
			b.Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.4))
		}
	case Secondary, Danger:
		border, bg, fg := theme.BorderStrong, theme.Fade(theme.Hover, 0), theme.Text
		if kind == Danger {
			fg = theme.Error
		}
		if hover {
			bg = theme.Hover
			border = theme.AccentLine
			if kind == Danger {
				border = theme.Fade(theme.Error, 0.45)
			}
		}
		b.Background(tint(b, "bg", bg)).Border(1, tint(b, "border", border)).TextColor(fg)
	}
	b.Children(func() {
		if svg != nil {
			ui.Icon(c, svg).FontSize(13)
		}
		ui.Text(c, label).FontSize(12.5).SingleLine()
		if hint != "" {
			ui.Text(c, hint).FontSize(12).Opacity(0.55).SingleLine()
		}
	})
	return Pressable(b)
}

// PrimaryButton is the accent button with an icon and a shortcut hint.
func PrimaryButton(c *ui.Context, svg *ui.SVG, label, hint string) ui.Element {
	return Button(c, Primary, svg, label, hint)
}

// GhostButton is an outlined secondary button.
func GhostButton(c *ui.Context, label string) ui.Element {
	return Button(c, Secondary, nil, label, "")
}

// DangerButton is an outlined button for destructive actions.
func DangerButton(c *ui.Context, label string) ui.Element {
	return Button(c, Danger, nil, label, "")
}

// IconButton is a small square button showing only an icon; on marks a
// toggle that is on.
func IconButton(c *ui.Context, svg *ui.SVG, label string, on bool) ui.Element {
	b := ui.ButtonBase(c).Size(28, 28).Radius(theme.RadiusControl - 1).Center().Shrink(0).
		Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	bg, border, fg := theme.Fade(theme.Hover, 0), theme.Fade(theme.BorderStrong, 0), theme.TextSoft
	switch {
	case on:
		border, fg = theme.BorderStrong, theme.Accent
	case b.Hovered():
		bg, fg = theme.Hover, theme.Text
	}
	b.Background(tint(b, "bg", bg)).Border(1, tint(b, "border", border)).TextColor(tint(b, "fg", fg))
	b.Children(func() { ui.Icon(c, svg).FontSize(14) })
	return Pressable(b)
}

// Input styles a text input: sunk below the surface, its outline warming
// to the accent while hovered or focused.
func Input(c *ui.Context, value *string, placeholder string) ui.Element {
	in := ui.TextInput(c, value).Height(theme.ControlHeight).PaddingX(11).Radius(theme.RadiusControl).
		Background(theme.Sunken).Placeholder(placeholder).Label(placeholder)
	border := theme.BorderStrong
	if in.Focused() || in.Hovered() {
		border = theme.AccentLine
	}
	in.Border(1, tint(in, "border", border))
	return in
}

// SearchField is a filter input with a search icon in front.
func SearchField(c *ui.Context, value *string, placeholder string, svg *ui.SVG) {
	box := ui.Row(c).FillWidth().Height(theme.ControlHeight).PaddingX(11).Gap(8).Radius(theme.RadiusControl).
		Background(theme.Sunken)
	border := theme.BorderStrong
	if box.FocusWithin() || box.Hovered() {
		border = theme.AccentLine
	}
	box.Border(1, tint(box, "border", border)).Children(func() {
		ui.Icon(c, svg).FontSize(13).TextColor(theme.Muted)
		ui.TextInputBase(c, value).Grow(1).FontSize(12).Placeholder(placeholder).Label(placeholder)
	})
}

// Trigger styles the face of a drop-down, open while its pop-up shows.
func Trigger(t ui.Element, open bool) ui.Element {
	t.FillWidth().Height(36).PaddingX(11).Gap(10).Radius(theme.RadiusControl).Background(theme.Sunken).Cursor(ui.CursorPointer)
	border := theme.BorderStrong
	if open || t.Hovered() {
		border = theme.AccentLine
	}
	return t.Border(1, tint(t, "border", border))
}

// Switch is an on/off toggle; it reports whether it changed.
func Switch(c *ui.Context, on *bool, label string) bool {
	// The knob sits on the left, centered top to bottom, and slides right.
	b := ui.ButtonBase(c).Size(32, 18).Radius(9).Shrink(0).Label(label).Cursor(ui.CursorPointer).
		Justify(ui.Start).AlignItems(ui.Center)
	track, border, knob := theme.Selected, theme.BorderStrong, theme.Muted
	if *on {
		track, border, knob = theme.AccentSoft, theme.AccentLine, theme.Accent
	}
	x := b.AnimateTo("x", map[bool]float32{false: 0, true: 14}[*on], Press)
	b.Background(tint(b, "track", track)).Border(1, tint(b, "border", border)).Children(func() {
		ui.Box(c).Size(12, 12).Radius(6).Shrink(0).Margin(0, 0, 0, 2+x).Background(tint(b, "knob", knob))
	})
	if b.Clicked() {
		*on = !*on
		return true
	}
	return false
}

// Checkbox is a box with its label; it reports whether it changed.
func Checkbox(c *ui.Context, checked *bool, label string) bool {
	b := ui.ButtonBase(c).Gap(8).Cursor(ui.CursorPointer).Label(label).Shrink(0)
	b.Children(func() {
		box := ui.Box(c).Size(15, 15).Radius(4).Center().TextColor(theme.OnAccent)
		if *checked {
			box.Background(theme.Accent)
		} else {
			border := theme.Idle
			if b.Hovered() {
				border = theme.AccentLine
			}
			box.Border(1, border)
		}
		box.Children(func() {
			if *checked {
				ui.Box(c).Size(7, 7).Radius(2).Background(theme.OnAccent)
			}
		})
		ui.Text(c, label).FontSize(12.5).TextColor(theme.TextSoft).SingleLine()
	})
	if b.Clicked() {
		*checked = !*checked
		return true
	}
	return false
}

// Field is a form field: a label over its input.
func Field(c *ui.Context, label string, fn func()) {
	ui.Column(c).FillWidth().Gap(7).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(theme.TextSoft)
		fn()
	})
}
