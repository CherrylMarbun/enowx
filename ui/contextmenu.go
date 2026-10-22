package ui

import "github.com/egoist/mygo/ui"

// contextMenu is the open right-click menu: which element it belongs to and
// where, relative to that element, the pointer was.
type contextMenu struct {
	key  string
	x, y float32
}

// openContextMenu opens key's menu at the pointer when anchor is
// right-clicked.
func (a *App) openContextMenu(anchor ui.Element, key string) {
	if !anchor.RightClicked() {
		return
	}
	x, y, _ := anchor.PointerPosition()
	a.menu = &contextMenu{key: key, x: x, y: y}
}

// showContextMenu draws key's menu, if open, at the pointer over anchor;
// items builds its entries with menuItem and menuSeparator.
func (a *App) showContextMenu(c *ui.Context, anchor ui.Element, key string, items func(done func())) {
	m := a.menu
	if m == nil || m.key != key {
		return
	}
	open := true
	closeMenu := func() { open = false }
	ui.PopoverBase(c, anchor, &open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorTopStart, ui.AnchorTopStart).Margin(m.y+4, 0, 0, m.x+4).
			MinWidth(190).Padding(5).Gap(1).Radius(10).Background(ui.Hex("#16181D")).Border(1, colBorderHi).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.45))
		items(closeMenu)
	})
	if !open && a.menu == m {
		a.menu = nil
	}
}

// menuItem is an entry of a context menu; danger colors destructive ones.
func menuItem(c *ui.Context, svg *ui.SVG, label string, danger bool) bool {
	color := colTextSoft
	if danger {
		color = colError
	}
	b := ui.ButtonBase(c).Gap(9).PaddingX(9).Height(30).Radius(6).TextColor(color).
		Cursor(ui.CursorPointer).Label(label).FillWidth().Justify(ui.Start)
	if b.Hovered() {
		b.Background(colSelected)
		if !danger {
			b.TextColor(colTextStrong)
		}
	}
	b.Children(func() {
		ui.Icon(c, svg).FontSize(13)
		ui.Text(c, label).FontSize(13).SingleLine()
	})
	return b.Clicked()
}

func menuSeparator(c *ui.Context) {
	ui.Box(c).Height(1).FillWidth().Margin(4, 0).Background(colBorderHi)
}
