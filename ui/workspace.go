package ui

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/chatview"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// headerHeight is the height of every view's top bar, so the breadcrumbs
// of the rail's views line up.
const headerHeight = 44

// The rail's views, shown instead of the sessions.
const (
	pageUsage      = "usage"
	pageSync       = "sync"
	pageHosts      = "hosts"
	pageAppearance = "appearance"
	pageCredits    = "credits"
)

// workspace is the main area: one focused session, or the project's
// sessions side by side in split mode, or a rail view.
func (a *App) workspace(c *ui.Context) {
	if a.page != pageHosts {
		a.hostsPageShown = false
	}
	// A new view (a rail page, another session) rises in as it replaces
	// the last; the panel itself moves with the rail.
	view := "page:" + a.page
	if a.page == "" {
		view = "session:" + a.selected
		if a.splitOn() {
			view = "split:" + a.project
		}
	}
	widget.Bento(ui.Column(c.Key("workspace")).Grow(1).FillHeight().MinWidth(0), theme.Background).
		Transition(widget.LayoutMotion).Children(func() {
		ui.Column(c.Key(view)).Grow(1).FillWidth().MinHeight(0).Transition(widget.PageMotion).Children(func() { a.workspaceView(c) })
	})
}

// workspaceView fills the workspace with what it shows now.
func (a *App) workspaceView(c *ui.Context) {
	switch {
	case a.page == pageAppearance:
		a.appearancePage(c)
		return
	case a.page == pageHosts:
		a.hostsPage(c)
		return
	case a.page == pageSync && a.sync != nil:
		a.syncPage(c)
		return
	case a.page == pageUsage && a.client != nil:
		a.usagePage(c)
		return
	case a.page == pageCredits && a.client != nil:
		a.creditsPage(c)
		return
	}
	a.page = ""
	if a.connectionView(c) {
		return
	}
	p, ok := a.currentProject()
	if !ok {
		a.viewHeader(c, func() { widget.Breadcrumb(c, a.host.Name, "Start") })
		a.emptyState(c, "Open a folder to start", "Each folder is a project. Sessions run in it or in their own git worktrees.", "Open folder", a.openFolder)
		return
	}
	sessions := a.projectSessions(p.Path)
	if len(sessions) == 0 {
		a.viewHeader(c, func() { widget.Breadcrumb(c, a.host.Name, p.Name) })
		a.emptyState(c, "No sessions in "+p.Name, "Start an agent or a shell. On a git project it can get its own branch and worktree.", "New session", func() { a.openNewSession(p) })
		return
	}
	if a.splitOn() && a.maximized == "" && len(sessions) > 1 {
		a.splitView(c, p, sessions)
		return
	}
	a.focusView(c, p, a.focused(sessions))
}

// focused is the session the focus view shows: the maximized one, else
// the selected one, else the project's first.
func (a *App) focused(sessions []proto.SessionInfo) proto.SessionInfo {
	for _, id := range []string{a.maximized, a.selected} {
		for _, s := range sessions {
			if id != "" && s.ID == id {
				return s
			}
		}
	}
	return sessions[0]
}

// viewHeader is the top bar of a view; fn fills it, breadcrumb first.
func (a *App) viewHeader(c *ui.Context, fn func()) {
	ui.Row(c).Height(headerHeight).Shrink(0).FillWidth().PaddingX(14).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(fn)
}

// sessionsHidden reports whether the Sessions panel gives way to a
// maximized session.
func (a *App) sessionsHidden() bool { return a.maximized != "" }

func (a *App) emptyState(c *ui.Context, title, body, action string, do func()) {
	widget.EmptyState(c, icon.FolderPlus, title, body, func() {
		if widget.PrimaryButton(c, icon.Plus, action, "").Clicked() {
			do()
		}
	})
}

// focusView shows one session: its header, a strip of facts, then its
// terminal or chat.
func (a *App) focusView(c *ui.Context, p proto.ProjectInfo, s proto.SessionInfo) {
	if a.selected == "" {
		a.selected = s.ID
	}
	a.viewHeader(c, func() {
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			widget.Breadcrumb(c, a.host.Name, p.Name, a.sessionName(s))
		})
		a.viewToggle(c, s)
		if len(a.projectSessions(p.Path)) > 1 && a.maximized == "" &&
			widget.IconButton(c, icon.Split, "Show "+p.Name+"'s sessions side by side", false).Clicked() {
			a.setSplit(true)
		}
		svg, label := icon.Maximize, "Maximize ("+modKey+"⇧↩)"
		if a.maximized != "" {
			svg, label = icon.Restore, "Restore ("+modKey+"⇧↩)"
		}
		if widget.IconButton(c, svg, label, a.maximized != "").Clicked() {
			a.toggleMaximized(s.ID)
		}
		more := widget.IconButton(c, icon.More, "More", false)
		a.menu.OpenBelow(more, "more:"+s.ID, 24)
		a.menu.Show(c, more, "more:"+s.ID, func(done func()) { a.sessionMenuItems(c, s, done) })
		a.panelButton(c)
	})
	a.factStrip(c, s)
	a.sessionBody(c, s)
}

// viewToggle switches a session between its terminal and its chat, for
// agents whose conversation enowX can read.
func (a *App) viewToggle(c *ui.Context, s proto.SessionInfo) {
	views := a.sessionViews(s)
	if len(views) < 2 {
		return
	}
	if i := widget.Segmented(c.Key("view"), views, slices.Index(views, a.currentView(s)), nil); i >= 0 {
		a.showView(s, views[i])
	}
}

func canChat(s proto.SessionInfo) bool { return chatview.Supported(s) }

// factStrip is the line under a focused session's header: its state,
// agent, branch, age and what it uses.
func (a *App) factStrip(c *ui.Context, s proto.SessionInfo) {
	st := statusOf(s)
	ui.Row(c).Height(30).Shrink(0).FillWidth().PaddingX(14).Gap(14).Background(theme.Panel).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			widget.Dot(c, st.color, st.hollow, 7)
			ui.Text(c, st.label).FontSize(11.5).TextColor(st.color).SingleLine()
		})
		fact := func(text string, col ui.Color) {
			ui.Text(c, text).FontSize(11.5).TextColor(col).SingleLine().Shrink(1)
		}
		fact(a.agentName(s.Agent), theme.TextSoft)
		switch {
		case s.Worktree && s.Branch != "":
			ui.Text(c, s.Branch+" · worktree").Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1)
		case s.Branch != "":
			ui.Text(c, s.Branch+" · project folder").Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1)
		}
		if !s.Created.IsZero() {
			fact(textfmt.Age(s.Created, time.Now()), theme.Muted)
		}
		if s.SkipPermissions {
			fact("skips permission prompts", theme.Warning)
		}
		if text, col := budgetFact(s); text != "" {
			fact(text, col)
		}
		ui.Spacer(c)
		if u, ok := a.usage.Session(s.ID); ok {
			fact(fmt.Sprintf("%s · %s · %d %s", textfmt.Bytes(u.RSS), textfmt.CPU(u.CPU), u.Procs, textfmt.Plural(u.Procs, "process", "processes")), theme.Muted)
		}
	})
}

// sessionBody is a session's chat or terminal, filling what is left.
func (a *App) sessionBody(c *ui.Context, s proto.SessionInfo) {
	switch p := a.panes[s.ID]; {
	case a.showsChat(s.ID):
		a.chatView(c.Key("chat"), s)
	case p != nil:
		v := terminal.View(c.Key("term"), p.term).Grow(1).FillWidth()
		if v.Focused() && a.selected != s.ID {
			a.selected = s.ID
		}
	default:
		ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
			ui.Text(c, "Connecting…").TextColor(theme.Muted)
		})
	}
}

// splitView lays the project's sessions out side by side, in the columns
// its layout says (as square a grid as fits them by default), leaving out
// those the user took out of it.
func (a *App) splitView(c *ui.Context, p proto.ProjectInfo, all []proto.SessionInfo) {
	sessions := a.splitSessions(all)
	l := a.layout()
	a.viewHeader(c, func() {
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			widget.Breadcrumb(c, a.host.Name, p.Name, fmt.Sprintf("Split · %d sessions", len(sessions)))
		})
		if left := len(all) - len(sessions); left > 0 &&
			widget.Button(c, widget.Secondary, nil, fmt.Sprintf("Show %d more", left), "").Clicked() {
			a.setLayout(func(l *layout) { l.Hidden = nil })
		}
		labels := make([]string, len(splitColumns))
		for i, n := range splitColumns {
			labels[i] = strconv.Itoa(n)
		}
		labels[0] = "Auto"
		if i := widget.Segmented(c.Key("columns"), labels, slices.Index(splitColumns, l.Columns), nil); i >= 0 {
			a.setLayout(func(l *layout) { l.Columns = splitColumns[i] })
		}
		if widget.IconButton(c, icon.Split, "Show one session", true).Clicked() {
			a.setSplit(false)
		}
		a.panelButton(c)
	})
	cols := l.Columns
	if cols <= 0 {
		cols = int(math.Ceil(math.Sqrt(float64(len(sessions)))))
	}
	cols = min(cols, len(sessions))
	rows := (len(sessions) + cols - 1) / cols
	tracks := make([]ui.Track, rows)
	for i := range tracks {
		tracks[i] = ui.Fr(1)
	}
	ui.Grid(c).Grow(1).FillWidth().Columns(cols).RowTracks(tracks...).Gap(1).
		Background(theme.Border).Children(func() {
		for _, s := range sessions {
			a.splitPane(c.Key(s.ID), s)
		}
	})
}

// splitPane is one session of the split view: a slim header over its
// terminal or chat. The selected one is marked by an accent line.
func (a *App) splitPane(c *ui.Context, s proto.SessionInfo) {
	st := statusOf(s)
	active := s.ID == a.selected
	ui.Column(c).MinHeight(0).MinWidth(0).Clip().Background(theme.Background).Children(func() {
		head := ui.Row(c).Height(34).Shrink(0).FillWidth().PaddingX(10).Gap(8).
			Background(theme.Panel).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border)
		if active {
			head.BorderWidth(2, 0, 1, 0).BorderColor(theme.Accent)
		}
		head.Children(func() {
			title := ui.ButtonBase(c).Grow(1).Gap(8).MinWidth(0).Justify(ui.Start).Cursor(ui.CursorPointer).Label(a.sessionName(s))
			title.Children(func() {
				widget.Dot(c, st.color, st.hollow, 7)
				ui.Text(c, a.sessionName(s)).FontSize(12).SingleLine().Shrink(1)
				ui.Text(c, a.agentName(s.Agent)).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1)
			})
			if title.Clicked() {
				a.selectSession(s)
			}
			a.viewToggle(c, s)
			if widget.IconButton(c, icon.Maximize, "Focus this session", false).Clicked() {
				a.selectSession(s)
				a.setSplit(false)
			}
			more := widget.IconButton(c, icon.More, "More", false)
			a.menu.OpenBelow(more, "more:"+s.ID, 24)
			a.menu.Show(c, more, "more:"+s.ID, func(done func()) { a.sessionMenuItems(c, s, done) })
		})
		a.sessionBody(c, s)
	})
}
