package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The Sessions panel folded to icons: each project a tile of its initial,
// each session its agent's icon with its status, their names in tooltips.
// It keeps the session list's clicks and menus.

const sessionsNarrow = 56

func (a *App) sessionListNarrow(c *ui.Context) {
	widget.Bento(ui.Column(c.Key("sessions")).Width(sessionsNarrow).Shrink(0).FillHeight(), theme.Panel).
		Transition(widget.PanelMotion(-24)).Children(func() {
		ui.Column(c).FillWidth().Padding(8, 0).Gap(6).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
			if widget.IconButton(c, icon.Sidebar, "Expand Sessions", false).Clicked() {
				a.state.SessionsNarrow = false
			}
			p, ok := a.currentProject()
			add := widget.IconButton(c, icon.Plus, "New session ("+modKey+"N)", false)
			if !ok {
				add.Opacity(0.45)
			} else if add.Clicked() {
				a.openNewSession(p)
			}
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(8, 0).Gap(4).AlignItems(ui.Center).Children(func() {
				first := true
				for _, e := range a.projectEntries() {
					for _, p := range e.projects {
						if !first {
							ui.Box(c).Size(24, 1).Margin(4, 0).Background(theme.Border)
						}
						first = false
						a.narrowProject(c, p)
					}
				}
			})
		})
	})
}

// narrowProject is a project's tile, selecting it, then its sessions.
func (a *App) narrowProject(c *ui.Context, p proto.ProjectInfo) {
	selected := p.Path == a.project
	tile := ui.ButtonBase(c.Key("tile:"+p.Path)).Size(32, 22).Radius(6).Center().Cursor(ui.CursorPointer).Label(p.Name).Tooltip(p.Name)
	if selected {
		tile.Background(theme.Hover)
	} else if tile.Hovered() {
		tile.Background(theme.Hover)
	}
	tile.Children(func() {
		initial := strings.ToUpper(string([]rune(p.Name + "?")[0]))
		col := theme.Muted
		if selected {
			col = theme.TextSoft
		}
		ui.Text(c, initial).FontSize(10.5).FontWeight(600).TextColor(col)
	})
	if widget.Pressable(tile).Clicked() {
		a.project, a.selected, a.page = p.Path, "", ""
	}
	a.menu.Open(tile, "project:"+p.Path)
	a.menu.Show(c, tile, "project:"+p.Path, func(done func()) { a.projectMenuItems(c, p, done) })
	for _, s := range a.projectSessions(p.Path) {
		a.narrowSession(c, s)
	}
}

// narrowSession is a session's icon: its agent, its status's dot, its
// name and state in the tooltip.
func (a *App) narrowSession(c *ui.Context, s proto.SessionInfo) {
	st := statusOf(s)
	active := s.ID == a.selected
	tip := a.sessionName(s) + " · " + st.label
	if s.Branch != "" && s.Branch != a.sessionName(s) {
		tip += " · " + s.Branch
	}
	b := ui.ButtonBase(c.Key("ns:"+s.ID)).Size(40, 40).Radius(theme.RadiusControl).Center().Cursor(ui.CursorPointer).Label(a.sessionName(s)).Tooltip(tip)
	switch {
	case active:
		b.Background(theme.Selected)
	case b.Hovered():
		b.Background(theme.Hover)
	}
	if a.picked[s.ID] && !active {
		b.Border(1, theme.AccentLine)
	}
	b.Children(func() {
		agentTile(c, s.Agent, 28)
		ui.Box(c).Size(10, 10).Radius(5).Background(theme.Panel).Center().
			AttachTo(b, ui.AnchorBottomRight, ui.AnchorBottomRight).Margin(0, 3, 3, 0).Children(func() {
			widget.Dot(c, st.color, st.hollow, 6)
		})
		if a.unread[s.ID] {
			ui.Box(c).Size(7, 7).Radius(4).Background(theme.Accent).AttachTo(b, ui.AnchorTopRight, ui.AnchorTopRight).Margin(3, 3, 0, 0)
		}
	})
	a.sessionMenu(c, b, s)
	if widget.Pressable(b).Clicked() {
		if b.ClickModifiers()&(ui.Cmd|ui.Shift) != 0 {
			a.pick(s)
		} else {
			a.picked = nil
			a.selectSession(s)
		}
	}
}
