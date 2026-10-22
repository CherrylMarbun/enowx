package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

func (a *App) sidebar(c *ui.Context) {
	ui.Column(c).Width(268).FillHeight().Background(colPanel).
		BorderWidth(0, 1, 0, 0).BorderColor(colBorder).Padding(12).Gap(14).Children(func() {
		a.hostSwitcher(c)

		ui.Row(c).PaddingX(6).Children(func() {
			ui.Text(c, "PROJECTS").FontSize(11).Bold().TextColor(colMuted)
			ui.Spacer(c)
			if iconButton(c, iconPlus, "Open folder as project", false).Clicked() {
				a.openFolder()
			}
		})

		ui.Scroll(c).Grow(1).FillWidth().Margin(-6, 0, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Gap(2).Children(func() {
				if len(a.projects) == 0 {
					ui.Text(c, "No projects yet. Open a folder to start.").FontSize(12).TextColor(colMuted).Padding(6)
				}
				for _, p := range a.projects {
					a.projectRow(c.Key(p.Path), p)
				}
			})
		})

		b := ui.ButtonBase(c).Height(34).FillWidth().Center().Gap(8).Radius(8).
			Border(1, colBorderHi).TextColor(colTextSoft).Cursor(ui.CursorPointer).Label("Open folder")
		if b.Hovered() {
			b.Background(colHover)
		}
		b.Children(func() {
			ui.Icon(c, iconFolderPlus).FontSize(14)
			ui.Text(c, "Open folder…").FontSize(13)
		})
		if b.Clicked() {
			a.openFolder()
		}
	})
}

func (a *App) projectRow(c *ui.Context, p proto.ProjectInfo) {
	sessions := a.projectSessions(p.Path)
	open := !a.collapsed[p.Path]
	selected := p.Path == a.project

	ui.Row(c).Height(32).FillWidth().Gap(4).Children(func() {
		row := ui.ButtonBase(c).Grow(1).Height(32).PaddingX(6).Gap(8).Radius(8).
			Cursor(ui.CursorPointer).Label(p.Name)
		if selected && a.selected == "" || row.Hovered() {
			row.Background(colHover)
		}
		row.Children(func() {
			chev := iconChevronRight
			if open {
				chev = iconChevronDown
			}
			ui.Icon(c, chev).FontSize(12).TextColor(colMuted)
			kind := iconFolder
			if p.IsGit {
				kind = iconGit
			}
			ui.Icon(c, kind).FontSize(14).TextColor(colTextSoft)
			name := ui.Text(c, p.Name).SingleLine().Grow(1)
			if selected {
				name.Bold().TextColor(colTextStrong)
			}
			if !open {
				ui.Text(c, summary(sessions)).FontSize(11).TextColor(colMuted).SingleLine()
			}
		})
		if row.Clicked() {
			if selected {
				a.collapsed[p.Path] = open
			} else {
				a.project = p.Path
				a.selected = ""
				a.maximized = ""
				delete(a.collapsed, p.Path)
			}
		}
		if iconButton(c, iconPlus, "New session in "+p.Name, true).Clicked() {
			a.openNewSession(p)
		}
	})

	if !open || len(sessions) == 0 {
		return
	}
	ui.Column(c).Gap(2).Margin(0, 0, 4, 12).Padding(0, 0, 0, 8).
		BorderWidth(0, 0, 0, 1).BorderColor(colBorderHi).Children(func() {
		for _, s := range sessions {
			a.sessionRow(c.Key(s.ID), s)
		}
	})
}

func (a *App) sessionRow(c *ui.Context, s proto.SessionInfo) {
	st := statusOf(s)
	active := s.ID == a.selected
	row := ui.ButtonBase(c).Padding(7, 10).Radius(8).Cursor(ui.CursorPointer).Label(a.sessionName(s))
	switch {
	case active:
		row.Background(colSelected)
	case row.Hovered():
		row.Background(colHover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				dot(c, st.color, st.hollow, 7)
				name := ui.Text(c, a.sessionName(s)).SingleLine().Grow(1)
				if active {
					name.Bold().TextColor(colTextStrong)
				}
				ui.Icon(c, agentIcon(s.Agent)).FontSize(13).TextColor(colMuted).Label(a.agentName(s.Agent))
			})
			parts := []string{}
			if s.Branch != "" && s.Branch != a.sessionName(s) {
				parts = append(parts, s.Branch)
			}
			if !s.Worktree && s.Project != "" {
				parts = append(parts, "project folder")
			}
			detail := strings.Join(append(parts, st.label), " · ")
			ui.Text(c, detail).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine().Margin(0, 0, 0, 15)
		})
	})
	a.openContextMenu(row, "session:"+s.ID)
	a.showContextMenu(c, row, "session:"+s.ID, func(done func()) {
		if menuItem(c, iconMaximize, "Maximize", false) {
			a.selectSession(s)
			a.maximized = s.ID
			done()
		}
		menuSeparator(c)
		if menuItem(c, iconTrash, "Delete session…", true) {
			a.askDelete(s)
			done()
		}
	})
	if row.Clicked() {
		a.selectSession(s)
	}
}

func summary(sessions []proto.SessionInfo) string {
	working := 0
	for _, s := range sessions {
		if s.Active && !s.Exited {
			working++
		}
	}
	switch {
	case working > 0:
		return fmt.Sprintf("%d working", working)
	case len(sessions) > 0:
		return fmt.Sprintf("%d", len(sessions))
	}
	return ""
}
