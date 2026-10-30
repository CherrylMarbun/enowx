package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The sessions as icons, in the folded sidebar: each project a tile of
// its initial, each session its agent's icon with its status, their names
// in tooltips. They keep the session list's clicks and menus.

// narrowProject is a project as a group: its initials on top (selecting
// it, its menu), its sessions under them, and in the current project's
// group the button starting a session there.
func (a *App) narrowProject(c *ui.Context, p proto.ProjectInfo) {
	selected := p.Path == a.project
	group := ui.Column(c.Key("group:"+p.Path)).Width(46).Padding(5, 0).Gap(3).Radius(13).AlignItems(ui.Center)
	if selected {
		group.Background(theme.Sunken).Border(1, theme.Border)
	}
	group.Children(func() {
		tile := ui.ButtonBase(c.Key("tile:"+p.Path)).Size(38, 18).Radius(5).Center().Cursor(ui.CursorPointer).
			Label(p.Name).Tooltip("Project · " + p.Name)
		if tile.Hovered() {
			tile.Background(theme.Hover)
		}
		tile.Children(func() {
			col := theme.Idle
			if selected {
				col = theme.TextSoft
			}
			ui.Text(c, projectInitials(p.Name)).FontSize(9.5).FontWeight(650).LetterSpacing(0.6).TextColor(col)
		})
		if widget.Pressable(tile).Clicked() {
			a.project, a.selected, a.page = p.Path, "", ""
		}
		a.menu.Open(tile, "project:"+p.Path)
		a.menu.Show(c, tile, "project:"+p.Path, func(done func()) { a.projectMenuItems(c, p, done) })
		for _, s := range a.projectSessions(p.Path) {
			a.narrowSession(c, s)
		}
		if selected {
			add := widget.IconButton(c.Key("new"), icon.Plus, "New session in "+p.Name+" ("+modKey+"N)", false)
			if add.Clicked() {
				a.openNewSession(p)
			}
		}
	})
}

// projectInitials are a project's name in two letters: the first two
// words' initials, or its first two letters.
func projectInitials(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == ' ' || r == '-' || r == '_' || r == '.' })
	var out []rune
	if len(words) >= 2 {
		out = []rune{[]rune(words[0])[0], []rune(words[1])[0]}
	} else {
		r := []rune(name)
		if len(r) == 0 {
			return "?"
		}
		out = r[:min(2, len(r))]
	}
	return strings.ToUpper(string(out))
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
	b := ui.ButtonBase(c.Key("ns:"+s.ID)).Size(38, 38).Radius(theme.RadiusControl).Center().Cursor(ui.CursorPointer).Label(a.sessionName(s)).Tooltip(tip)
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
		agentTile(c, s.Agent, 26)
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
