package widget

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// ContextMenu is the open right-click menu of a window: which element it
// belongs to and where, relative to that element, the pointer was. The
// zero value has no menu open. It belongs to the main thread.
type ContextMenu struct {
	open *menuAt
}

type menuAt struct {
	key  string
	x, y float32
}

// Open opens key's menu at the pointer when anchor is right-clicked.
func (m *ContextMenu) Open(anchor ui.Element, key string) {
	if !anchor.RightClicked() {
		return
	}
	x, y, _ := anchor.PointerPosition()
	m.open = &menuAt{key: key, x: x, y: y}
}

// OpenBelow opens key's menu under anchor when anchor is clicked, for
// buttons that show a menu; height is anchor's height.
func (m *ContextMenu) OpenBelow(anchor ui.Element, key string, height float32) {
	if anchor.Clicked() {
		m.open = &menuAt{key: key, y: height}
	}
}

// Show draws key's menu, if open, at the pointer over anchor; items builds
// its entries with MenuItem and MenuSeparator, and calls done to close it.
func (m *ContextMenu) Show(c *ui.Context, anchor ui.Element, key string, items func(done func())) {
	at := m.open
	if at == nil || at.key != key {
		return
	}
	open := true
	closeMenu := func() { open = false }
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		Popup(panel.AttachTo(anchor, ui.AnchorTopStart, ui.AnchorTopStart).Margin(at.y+4, 0, 0, at.x+4).MinWidth(190))
		items(closeMenu)
	})
	if !open && m.open == at {
		m.open = nil
	}
}

// MenuItem is an entry of a context menu; danger colors destructive ones.
func MenuItem(c *ui.Context, svg *ui.SVG, label string, danger bool) bool {
	color := theme.TextSoft
	if danger {
		color = theme.Error
	}
	b := ui.ButtonBase(c).Gap(9).PaddingX(9).Height(30).Radius(theme.RadiusControl - 1).
		Cursor(ui.CursorPointer).Label(label).FillWidth().Justify(ui.Start)
	bg := theme.Fade(theme.Selected, 0)
	if b.Hovered() {
		bg = theme.Selected
		if !danger {
			color = theme.Text
		}
	}
	b.Background(tint(b, "bg", bg)).TextColor(color)
	b.Children(func() {
		ui.Icon(c, svg).FontSize(13)
		ui.Text(c, label).FontSize(12.5).SingleLine()
	})
	return b.Clicked()
}

// MenuSeparator separates groups of menu items.
func MenuSeparator(c *ui.Context) { Divider(c) }
