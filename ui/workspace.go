package ui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

func (a *App) workspace(c *ui.Context) {
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
		if a.connectionView(c) {
			return
		}
		p, ok := a.currentProject()
		if !ok {
			a.emptyState(c, "Open a folder to start", "Each folder is a project. Sessions run in it or in their own git worktrees.", "Open folder…", a.openFolder)
			return
		}
		sessions := a.projectSessions(p.Path)
		if a.maximized != "" {
			for _, s := range sessions {
				if s.ID == a.maximized {
					a.maximizedView(c, p, s, sessions)
					return
				}
			}
			a.maximized = ""
		}
		a.projectHeader(c, p, sessions)
		if len(sessions) == 0 {
			a.emptyState(c, "No sessions in "+p.Name, "Start an agent or a shell. On a git project it can get its own branch and worktree.", "New session", func() { a.openNewSession(p) })
			return
		}
		a.grid(c, sessions)
	})
}

func (a *App) emptyState(c *ui.Context, title, body, action string, do func()) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
		ui.Text(c, title).FontSize(16).Bold()
		ui.Text(c, body).TextColor(colMuted).MaxWidth(420).TextAlign(ui.Center)
		ui.Box(c).Height(6)
		if primaryButton(c, iconPlus, action, "").Clicked() {
			do()
		}
	})
}

func (a *App) projectHeader(c *ui.Context, p proto.ProjectInfo, sessions []proto.SessionInfo) {
	ui.Row(c).Height(52).FillWidth().PaddingX(16).Gap(12).BorderWidth(0, 0, 1, 0).BorderColor(colBorder).Children(func() {
		ui.Column(c).Gap(1).MinWidth(0).Children(func() {
			ui.Text(c, p.Name).FontSize(15).Bold().SingleLine()
			sub := p.Path
			if p.Branch != "" {
				sub += " · " + p.Branch
			}
			ui.Text(c, sub).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
		})
		counts := map[string]int{}
		for _, s := range sessions {
			counts[statusOf(s).label]++
		}
		for _, chip := range []struct {
			label string
			fg    ui.Color
			bg    ui.Color
		}{
			{"working", colWorking, ui.Hex("#182033")},
			{"idle", colTextSoft, colRaised},
			{"done", colDone, ui.Hex("#13261D")},
		} {
			if n := counts[chip.label]; n > 0 {
				ui.Row(c).Shrink(0).Gap(6).Padding(3, 9).Radius(999).Background(chip.bg).Children(func() {
					dot(c, chip.fg, chip.label == "idle", 6)
					ui.Text(c, fmt.Sprintf("%d %s", n, chip.label)).FontSize(12).TextColor(chip.fg).SingleLine()
				})
			}
		}
		ui.Spacer(c)
		if primaryButton(c, iconPlus, "New session", modKey+"N").Clicked() {
			a.openNewSession(p)
		}
	})
}

// grid lays the sessions out in as square a grid as fits them.
func (a *App) grid(c *ui.Context, sessions []proto.SessionInfo) {
	cols := int(math.Ceil(math.Sqrt(float64(len(sessions)))))
	rows := (len(sessions) + cols - 1) / cols
	tracks := make([]ui.Track, rows)
	for i := range tracks {
		tracks[i] = ui.Fr(1)
	}
	ui.Grid(c).Grow(1).FillWidth().Columns(cols).RowTracks(tracks...).Gap(8).Padding(8).
		Background(ui.Hex("#0B0C0F")).Children(func() {
		for _, s := range sessions {
			a.paneCard(c.Key(s.ID), s, false)
		}
	})
}

// paneCard is one session: a header and its terminal.
func (a *App) paneCard(c *ui.Context, s proto.SessionInfo, maximized bool) {
	st := statusOf(s)
	active := s.ID == a.selected
	card := ui.Column(c).Radius(10).Clip().Background(colBackground).MinHeight(0)
	if maximized {
		card.Grow(1).FillWidth()
	} else if active {
		card.Border(1, colAccent)
	} else {
		card.Border(1, ui.Hex("#23272F"))
	}
	card.Children(func() {
		head := ui.Row(c).Height(36).FillWidth().PaddingX(12).Gap(8).
			Background(colPanel).BorderWidth(0, 0, 1, 0).BorderColor(colBorder)
		if active && !maximized {
			head.Background(ui.Hex("#141822"))
		}
		head.Children(func() {
			title := ui.ButtonBase(c).Grow(1).Gap(8).MinWidth(0).Justify(ui.Start).Cursor(ui.CursorPointer).Label(a.sessionName(s))
			title.Children(func() {
				dot(c, st.color, st.hollow, 7)
				ui.Text(c, a.sessionName(s)).Bold().FontSize(12).SingleLine()
				if s.Branch != "" && s.Branch != a.sessionName(s) {
					ui.Text(c, s.Branch).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
				}
			})
			if title.Clicked() {
				a.selectSession(s)
			}
			ui.Row(c).Gap(6).Children(func() {
				ui.Icon(c, agentIcon(s.Agent)).FontSize(12).TextColor(colTextSoft)
				where := a.agentName(s.Agent)
				if !s.Worktree && s.Project != "" {
					where += " · project folder"
				}
				ui.Text(c, where).FontSize(12).TextColor(ui.Hex("#A3A8B2")).SingleLine()
			})
			if s.Exited {
				if iconButton(c, iconClose, "Close session", false).Clicked() {
					a.killSession(s.ID)
				}
			}
			svg, label := iconMaximize, "Maximize ("+modKey+"⇧↩)"
			if maximized {
				svg, label = iconRestore, "Restore grid ("+modKey+"⇧↩)"
			}
			if iconButton(c, svg, label, active || maximized).Clicked() {
				a.toggleMaximized(s.ID)
			}
		})
		if p := a.panes[s.ID]; p != nil {
			v := terminal.View(c.Key("term"), p.term).Grow(1).FillWidth()
			if v.Focused() && a.selected != s.ID {
				a.selected = s.ID
			}
		} else {
			ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
				ui.Text(c, "Connecting…").TextColor(colMuted)
			})
		}
	})
}

// maximizedView shows one session over the whole workspace. Its header
// keeps the grid's height, so the restore button sits exactly where the
// maximize button was; the project's other sessions become chips.
func (a *App) maximizedView(c *ui.Context, p proto.ProjectInfo, s proto.SessionInfo, sessions []proto.SessionInfo) {
	ui.Row(c).Height(52).FillWidth().PaddingX(16).Gap(4).BorderWidth(0, 0, 1, 0).BorderColor(colBorder).Children(func() {
		ui.Text(c, p.Name).FontSize(15).Bold().SingleLine().Margin(0, 10, 0, 0)
		for _, o := range sessions {
			if o.ID == s.ID {
				continue
			}
			st := statusOf(o)
			chip := ui.ButtonBase(c.Key(o.ID)).Shrink(0).Gap(6).Padding(5, 9).Radius(6).Cursor(ui.CursorPointer).Label(a.sessionName(o))
			if chip.Hovered() {
				chip.Background(colHover)
			}
			chip.Children(func() {
				dot(c, st.color, st.hollow, 6)
				ui.Text(c, a.sessionName(o)).FontSize(12).TextColor(colTextSoft).SingleLine()
			})
			if chip.Clicked() {
				a.selectSession(o)
			}
		}
		ui.Spacer(c)
		if primaryButton(c, iconPlus, "New session", modKey+"N").Clicked() {
			a.openNewSession(p)
		}
	})
	ui.Column(c).Grow(1).FillWidth().Padding(8).Background(ui.Hex("#0B0C0F")).Children(func() {
		a.paneCard(c.Key(s.ID), s, true)
	})
}
