package widget

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Backdrop dims what is under a dialog.
var Backdrop = ui.RGBA(0, 0, 0, 0.62)

// tint animates one of e's colors toward target.
func tint(e ui.Element, key string, target ui.Color) ui.Color {
	return e.AnimateColor(key, target, ease)
}

// Tint eases e's color named key toward target, at the pace the widgets
// change theirs.
func Tint(e ui.Element, key string, target ui.Color) ui.Color { return tint(e, key, target) }

// Raise gives e the look of a surface above its ground: a faint light
// along its top edge and a soft shadow under it.
func Raise(e ui.Element) ui.Element {
	return e.InsetShadow(0, 1, 0, 0, theme.Highlight).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.32))
}

// Bento makes e one of the window's panels: rounded, outlined, apart from
// the others on the ground.
func Bento(e ui.Element, bg ui.Color) ui.Element {
	return e.Radius(theme.RadiusBento).Clip().Background(bg).Border(1, theme.Border).
		InsetShadow(0, 1, 0, 0, theme.Highlight)
}

// Float gives e the deep shadow of an overlay: menus, pop-ups, dialogs.
func Float(e ui.Element) ui.Element {
	return e.InsetShadow(0, 1, 0, 0, theme.Highlight).Shadow(0, 24, 60, 0, ui.RGBA(0, 0, 0, 0.6))
}

// Popup styles the panel of a drop-down or pop-over, which grows in.
func Popup(panel ui.Element) ui.Element {
	return Float(panel.Padding(6).Gap(1).Radius(theme.RadiusCard).Background(theme.Dialog).Border(1, theme.BorderStrong)).
		TransformOrigin(0.5, 0).Transition(OverlayMotion)
}

// PopupItem styles one entry of a pop-up: highlighted while hovered or
// picked with the keyboard.
func PopupItem(item ui.Element, highlighted bool) ui.Element {
	item.FillWidth().Gap(10).Padding(7, 9).Radius(theme.RadiusControl - 1)
	bg := theme.Fade(theme.Selected, 0)
	if highlighted {
		bg = theme.Selected
	}
	return item.Background(tint(item, "bg", bg))
}

// PopupLabel is the title of a group of pop-up entries.
func PopupLabel(c *ui.Context, label string) {
	SectionLabel(c, label, theme.Muted).Padding(8, 9, 4, 9)
}

// Divider is a hairline between groups.
func Divider(c *ui.Context) {
	ui.Box(c).Height(1).FillWidth().Margin(4, 2).Background(theme.Border)
}
