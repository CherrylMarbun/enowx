package ui

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo/ui"
)

// modKey is how shortcut hints write ui.Cmd.
var modKey = func() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "Ctrl+"
}()

func (a *App) view(c *ui.Context) {
	c.SetTheme(a.theme)
	a.shortcuts(c)
	ui.Column(c).Fill().Background(colBackground).TextColor(colText).Children(func() {
		ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			a.workspace(c)
		})
		a.statusBar(c)
	})
	a.newSessionDialog(c)
	a.addProjectDialog(c)
	a.addHostDialog(c)
	a.manageHostsDialog(c)
	a.trustDialog(c)
	a.runAsDialog(c)
	a.passphraseDialog(c)
	a.signInDialog(c)
	a.syncDialog(c)
	a.usageDialog(c)
	a.deleteDialog(c)
	a.saveState()
}

func (a *App) shortcuts(c *ui.Context) {
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		if p, ok := a.currentProject(); ok {
			a.openNewSession(p)
		}
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEnter) {
		a.toggleMaximized(a.selected)
	}
	if c.Shortcut(ui.Cmd, ui.KeyO) {
		a.openFolder()
	}
}

func (a *App) toggleMaximized(id string) {
	switch {
	case a.maximized != "":
		a.maximized = ""
	case id != "":
		a.maximized = id
		a.selected = id
	}
}

func (a *App) statusBar(c *ui.Context) {
	working, idle := 0, 0
	for _, s := range a.sessions {
		switch {
		case s.Exited:
		case s.Active:
			working++
		default:
			idle++
		}
	}
	ui.Row(c).Height(28).FillWidth().PaddingX(14).Gap(16).Background(colPanel).
		BorderWidth(1, 0, 0, 0).BorderColor(colBorder).Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			col, _ := a.connectionState()
			dot(c, col, false, 6)
			ui.Text(c, a.host.Name).FontSize(11).TextColor(colMuted)
		})
		ui.Text(c, fmt.Sprintf("%d working · %d idle", working, idle)).FontSize(11).TextColor(colMuted)
		if a.status != "" {
			col := colError
			if a.status == a.notice {
				col = colTextSoft
			}
			ui.Text(c, a.status).FontSize(11).TextColor(col).SingleLine().Grow(1)
		} else {
			ui.Spacer(c)
		}
		a.usageButton(c)
		ui.Text(c, modKey+"N new session · "+modKey+"⇧↩ maximize · "+modKey+"O open folder").FontSize(11).TextColor(colMuted)
	})
}

// dot draws a session status mark: filled, or a ring when hollow.
// notify shows news that isn't a problem in the status bar.
func (a *App) notify(msg string) { a.status, a.notice = msg, msg }

func dot(c *ui.Context, col ui.Color, hollow bool, size float32) {
	d := ui.Box(c).Size(size, size).Radius(size / 2)
	if hollow {
		d.Border(1.5, col)
	} else {
		d.Background(col)
	}
}

// iconButton is a small square button showing only an icon.
func iconButton(c *ui.Context, svg *ui.SVG, label string, filled bool) ui.Element {
	b := ui.ButtonBase(c).Size(24, 24).Radius(6).Center().TextColor(colTextSoft).
		Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	if filled || b.Hovered() {
		b.Background(colSelected)
	}
	b.Children(func() { ui.Icon(c, svg).FontSize(13) })
	return b
}

// primaryButton is the accent button with an icon and a shortcut hint.
func primaryButton(c *ui.Context, svg *ui.SVG, label, hint string) ui.Element {
	b := ui.ButtonBase(c).Gap(6).PaddingX(12).Height(30).Radius(7).Background(colAccent).
		TextColor(colOnAccent).Cursor(ui.CursorPointer).Label(label)
	b.Children(func() {
		ui.Icon(c, svg).FontSize(13)
		ui.Text(c, label).Bold().FontSize(13).SingleLine()
		if hint != "" {
			ui.Text(c, hint).FontSize(12).Opacity(0.65).SingleLine()
		}
	})
	return b
}

// ghostButton is an outlined secondary button.
func ghostButton(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c).PaddingX(11).Height(30).Radius(7).Border(1, colBorderHi).
		TextColor(colTextSoft).Cursor(ui.CursorPointer).Label(label)
	if b.Hovered() {
		b.Background(colHover)
	}
	b.Children(func() { ui.Text(c, label).FontSize(13).SingleLine() })
	return b
}

// deleteDialog confirms deleting a session.
func (a *App) deleteDialog(c *ui.Context) {
	if a.deleting == nil {
		return
	}
	s := *a.deleting
	open := true
	msg := "Its worktree and branch stay on disk."
	if !s.Exited {
		msg = "The agent stops. " + msg
	}
	switch ui.AlertDialog(c, &open, "Delete “"+a.sessionName(s)+"”?", msg, "Cancel", "Delete") {
	case 1:
		a.killSession(s.ID)
		a.deleting = nil
	case 0, ui.AlertDismissed:
		a.deleting = nil
	}
}
