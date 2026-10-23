package ui

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The side panel sits right of the workspace, about the selected session,
// and folds away. It holds tools as tabs: Git first; the session's files
// and a browser are meant to join it.

// panelTool is a tool the side panel can open as a tab.
type panelTool struct {
	id, name string
	svg      *ui.SVG
}

// Tools of the side panel, in the order its launchers show them.
const (
	panelChanges = "git"
	panelFiles   = "files"
	panelPorts   = "ports"
)

var panelTools = []panelTool{
	{panelChanges, "Git", icon.Git},
	{panelFiles, "Files", icon.Folder},
	{panelPorts, "Ports", icon.Globe},
}

func findTool(id string) (panelTool, bool) {
	i := slices.IndexFunc(panelTools, func(t panelTool) bool { return t.id == id })
	if i < 0 {
		return panelTool{}, false
	}
	return panelTools[i], true
}

// sidePanel is the side panel's state, remembered between launches.
type sidePanel struct {
	Open  bool     `json:"open,omitempty"`
	Tabs  []string `json:"tabs,omitempty"`  // the tools open, in tab order
	Tab   string   `json:"tab,omitempty"`   // the tab in front
	Width float32  `json:"width,omitempty"` // in DIPs; 0 for the default
}

// The side panel's width: as wide as the user dragged it, within these.
// A narrow window shrinks it down to its minimum before the workspace.
const (
	defaultPanelWidth = 460
	minPanelWidth     = 300
	maxPanelWidth     = 1100
	panelGrip         = 6 // the strip along its left edge that resizes it
)

// panelTabs are the tabs open, of tools this build has.
func (a *App) panelTabs() []string {
	return slices.DeleteFunc(slices.Clone(a.state.Panel.Tabs), func(id string) bool {
		_, ok := findTool(id)
		return !ok
	})
}

// panelTab is the tab in front, "" when none is open.
func (a *App) panelTab() string {
	tabs := a.panelTabs()
	if slices.Contains(tabs, a.state.Panel.Tab) {
		return a.state.Panel.Tab
	}
	if len(tabs) > 0 {
		return tabs[0]
	}
	return ""
}

// panelShown reports whether the side panel shows: open, beside a
// session's workspace (not a rail page).
func (a *App) panelShown() bool { return a.state.Panel.Open && a.page == "" && a.client != nil }

// panelShows reports whether the side panel shows tool in front.
func (a *App) panelShows(tool string) bool { return a.panelShown() && a.panelTab() == tool }

// openPanel shows the panel with tool's tab in front, opening the tab.
func (a *App) openPanel(tool string) {
	p := &a.state.Panel
	a.page = ""
	p.Open, p.Tab = true, tool
	if !slices.Contains(p.Tabs, tool) {
		p.Tabs = append(slices.Clone(p.Tabs), tool)
	}
}

// togglePanel folds the panel, or shows it as it was, on its first tool
// when no tab is open.
func (a *App) togglePanel() {
	if a.panelShown() {
		a.state.Panel.Open = false
		return
	}
	tool := a.panelTab()
	if tool == "" {
		tool = panelTools[0].id
	}
	a.openPanel(tool)
}

// toggleTool shows tool in the panel, or folds the panel if it shows it.
func (a *App) toggleTool(tool string) {
	if a.panelShows(tool) {
		a.state.Panel.Open = false
		return
	}
	a.openPanel(tool)
}

// closeTab closes tool's tab; the panel folds with its last one.
func (a *App) closeTab(tool string) {
	p := &a.state.Panel
	p.Tabs = slices.DeleteFunc(slices.Clone(p.Tabs), func(id string) bool { return id == tool })
	if len(a.panelTabs()) == 0 {
		p.Open = false
	}
}

// sidePanelView is the panel: its tab strip, then the tool in front, for
// the selected session. It slides in from the right as the session list
// does from the left, and its left edge resizes it.
func (a *App) sidePanelView(c *ui.Context) {
	if !a.panelShown() {
		return
	}
	width := a.state.Panel.Width
	if width <= 0 {
		width = defaultPanelWidth
	}
	widget.Bento(ui.Column(c.Key("side-panel")).Width(width).MinWidth(minPanelWidth).Shrink(1).FillHeight(), theme.Background).
		Transition(widget.PanelMotion(24)).Children(func() {
		a.panelStrip(c)
		s, ok := a.selectedSession()
		switch tool := a.panelTab(); {
		case !ok:
			panelNote(c, "Select a session to see its tools.")
		case tool == panelChanges && !a.canReview(s):
			panelNote(c, "“"+a.sessionName(s)+"” isn't in a git repository.")
		case tool == panelChanges:
			a.changesTab(c.Key("changes:"+s.ID), s)
		case tool == panelFiles:
			a.filesTab(c.Key("files:"+s.ID), s)
		case tool == panelPorts:
			a.portsTab(c.Key("ports:"+s.ID), s)
		}
		a.panelGrip(c, width)
	})
}

// panelGrip is the panel's left edge, which the pointer drags to resize
// it; it shows a line only while hovered or dragged.
func (a *App) panelGrip(c *ui.Context, width float32) {
	grip := ui.Box(c).Absolute().Left(0).Top(0).Bottom(0).Width(panelGrip).Cursor(ui.CursorResizeEW).Label("Resize the panel")
	dx, _, held := grip.Dragged()
	if held && dx != 0 {
		a.state.Panel.Width = min(max(width-dx, minPanelWidth), maxPanelWidth)
	}
	if held || grip.Hovered() {
		grip.Children(func() {
			ui.Box(c).Width(2).FillHeight().Background(theme.Accent)
		})
	}
}

// panelStrip is the panel's top row: a launcher per tool, the open tabs,
// a + to open one, and the button that folds the panel.
func (a *App) panelStrip(c *ui.Context) {
	front := a.panelTab()
	ui.Row(c).Height(40).Shrink(0).FillWidth().PaddingX(8).Gap(4).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		for _, t := range panelTools {
			if widget.IconButton(c.Key("tool:"+t.id), t.svg, t.name, false).Clicked() {
				a.openPanel(t.id)
			}
		}
		ui.Box(c).Size(1, 18).Background(theme.Border).Margin(0, 4)
		for _, id := range a.panelTabs() {
			t, _ := findTool(id)
			a.panelTabButton(c.Key("tab:"+id), t, id == front)
		}
		add := widget.IconButton(c, icon.Plus, "Open a tool", false)
		a.menu.OpenBelow(add, "panel-tools", 28)
		a.menu.Show(c, add, "panel-tools", func(done func()) {
			for _, t := range panelTools {
				if widget.MenuItem(c, t.svg, t.name, false) {
					a.openPanel(t.id)
					done()
				}
			}
		})
		ui.Spacer(c)
		if widget.IconButton(c, icon.PanelRight, "Hide the panel ("+modKey+"⇧D)", true).Clicked() {
			a.state.Panel.Open = false
		}
	})
}

// panelTabButton is an open tab: its tool, and an × that closes it.
func (a *App) panelTabButton(c *ui.Context, t panelTool, front bool) {
	b := ui.ButtonBase(c).Height(28).PaddingX(9).Gap(6).Radius(theme.RadiusControl).Cursor(ui.CursorPointer).Label(t.name).Shrink(0)
	bg, fg := theme.Fade(theme.Selected, 0), theme.Muted
	switch {
	case front:
		bg, fg = theme.Selected, theme.Accent
		b.Border(1, theme.BorderStrong)
	case b.Hovered():
		fg = theme.Text
	}
	b.Background(widget.Tint(b, "bg", bg)).TextColor(widget.Tint(b, "fg", fg))
	closed := false
	b.Children(func() {
		ui.Icon(c, t.svg).FontSize(13)
		ui.Text(c, t.name).FontSize(12).SingleLine()
		x := ui.ButtonBase(c).Size(16, 16).Radius(4).Center().Cursor(ui.CursorPointer).Label("Close " + t.name)
		if x.Hovered() {
			x.Background(theme.Hover)
		}
		x.Children(func() { ui.Icon(c, icon.Close).FontSize(10).TextColor(theme.Muted) })
		closed = x.Clicked()
	})
	switch {
	case closed:
		a.closeTab(t.id)
	case b.Clicked():
		a.state.Panel.Tab = t.id
	}
}

func panelNote(c *ui.Context, text string) {
	ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() {
		ui.Text(c, text).FontSize(12.5).TextColor(theme.Muted).TextAlign(ui.Center)
	})
}

// panelButton shows the side panel, from a view's header, while it is
// folded; the panel's own strip folds it again.
func (a *App) panelButton(c *ui.Context) {
	if !a.panelShown() && widget.IconButton(c, icon.PanelRight, "Show the panel ("+modKey+"⇧D)", false).Clicked() {
		a.togglePanel()
	}
}
