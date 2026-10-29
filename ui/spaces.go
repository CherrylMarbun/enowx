package ui

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Spaces (docs/cloud.md, Personal and team): the rail's picker says whose
// notes and canvases the Notes and Canvas pages show, the user's own or a
// team's. The connected host keeps each team's apart and lists them all,
// each marked with its team; the pages show the chosen space's, and what
// is made there is that space's. The picker shows once the host is signed
// in to the cloud as a member of a team.

// hostSpaces is the connected host's teams' spaces, as last listed. It
// belongs to the main thread.
type hostSpaces struct {
	host string
	list []proto.Space
}

const spacePersonal = "personal"

func (a *App) spacesOf() *hostSpaces {
	if a.spaceList == nil || a.spaceList.host != a.host.ID {
		a.spaceList = &hostSpaces{host: a.host.ID}
		hs, cl := a.spaceList, a.client
		if cl != nil {
			go func() {
				list, err := cl.Spaces()
				if err != nil {
					return
				}
				a.onMain(func() { hs.list = list })
			}()
		}
	}
	return a.spaceList
}

// spacesEvent takes the host's spaces as they changed.
func (a *App) spacesEvent(data json.RawMessage) func() {
	var list []proto.Space
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		if hs := a.spaceList; hs != nil && hs.host == a.host.ID {
			hs.list = list
		}
	}
}

// space is the team whose space the pages show, "" for the user's own:
// the one picked while the host has it.
func (a *App) space() string {
	team := a.state.Space
	if team == "" || !slices.ContainsFunc(a.spacesOf().list, func(sp proto.Space) bool { return sp.Team == team }) {
		return ""
	}
	return team
}

// spaceName is team's name as the host knows it, "" for the user's own.
func (a *App) spaceName(team string) string {
	if team == "" {
		return ""
	}
	for _, sp := range a.spacesOf().list {
		if sp.Team == team {
			return sp.Name
		}
	}
	return "a team"
}

// showSpace makes the pages show team's space, as opening one of its
// items does.
func (a *App) showSpace(team string) { a.state.Space = team }

// openNote shows note id on the Notes page, in team's space.
func (a *App) openNote(id, team string) {
	a.showSpace(team)
	a.page = pageNotes
	a.notes.Select(a.notesOptions(), id)
}

// notesInSpace are the notes of the space shown.
func (a *App) notesInSpace(list []proto.NoteInfo) []proto.NoteInfo {
	team := a.space()
	out := make([]proto.NoteInfo, 0, len(list))
	for _, n := range list {
		if n.Team == team {
			out = append(out, n)
		}
	}
	return out
}

// groupsInSpace are the groups of the space shown.
func (a *App) groupsInSpace(list []proto.NoteGroup) []proto.NoteGroup {
	team := a.space()
	out := make([]proto.NoteGroup, 0, len(list))
	for _, g := range list {
		if g.Team == team {
			out = append(out, g)
		}
	}
	return out
}

// canvasesInSpace are the canvases of the space shown.
func (a *App) canvasesInSpace(list []proto.CanvasInfo) []proto.CanvasInfo {
	team := a.space()
	out := make([]proto.CanvasInfo, 0, len(list))
	for _, cv := range list {
		if cv.Team == team {
			out = append(out, cv)
		}
	}
	return out
}

// railSpace is the rail's picker of spaces, over Notes and Canvas, which
// show the space picked: the user's own, or a team's.
func (a *App) railSpace(c *ui.Context, wide bool) {
	if a.client == nil {
		return
	}
	list := a.spacesOf().list
	if len(list) == 0 {
		return
	}
	team := a.space()
	name := "Personal"
	if team != "" {
		name = a.spaceName(team)
	}
	choice := ""
	sel := ui.SelectBase(c.Key("space"), &choice)
	trig := widget.RailEntry(sel.Trigger, wide).Cursor(ui.CursorPointer)
	if sel.Open() || trig.Hovered() {
		trig.Background(theme.Hover)
	}
	trig.Label("Space: " + name)
	if !wide {
		trig.Tooltip("Space: " + name + " · Notes and Canvas show its pages")
	}
	mark := func(team, name string, size float32) {
		if team == "" {
			ui.Box(c).Size(size, size).Radius(size/4 + 1).Shrink(0).Background(theme.Hover).Center().Children(func() {
				ui.Icon(c, icon.Account).FontSize(size * 0.62).TextColor(theme.TextSoft)
			})
			return
		}
		widget.TeamMark(c, team, name, size)
	}
	trig.Children(func() {
		mark(team, name, widget.RailIconSize+2)
		if wide {
			ui.Text(c, name).FontSize(12.5).TextColor(theme.Text).SingleLine().Shrink(1)
			ui.Spacer(c)
			ui.Icon(c, icon.ChevronDown).FontSize(11).TextColor(theme.Muted)
		}
	})
	sel.Popup(func(panel ui.Element) {
		widget.Popup(panel.AttachTo(trig, ui.AnchorTopRight, ui.AnchorTopLeft).Margin(0, 0, 0, 10).Width(240))
		item := func(id, team, label, sub string, on bool) {
			it := sel.Item(id)
			widget.PopupItem(it, it.Highlighted()).TextColor(theme.TextSoft).Children(func() {
				ui.Box(c).Size(28, 28).Center().Children(func() { mark(team, label, 18) })
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
					ui.Text(c, label).FontSize(12.5).TextColor(theme.Text).SingleLine()
					if sub != "" {
						ui.Text(c, sub).FontSize(11).TextColor(theme.Muted).SingleLine()
					}
				})
				if on {
					ui.Icon(c, icon.Check).FontSize(13).TextColor(theme.Accent)
				}
			})
		}
		widget.PopupLabel(c, "SPACE")
		item(spacePersonal, "", "Personal", "Only you, on your devices", team == "")
		for _, sp := range list {
			item("team:"+sp.Team, sp.Team, sp.Name, "Team", team == sp.Team)
		}
		widget.Divider(c)
		it := sel.Item("teams")
		widget.PopupItem(it, it.Highlighted()).TextColor(theme.TextSoft).Children(func() {
			ui.Box(c).Size(28, 28).Center().Children(func() { ui.Icon(c, icon.Settings).FontSize(14) })
			ui.Text(c, "Teams…")
		})
	})
	switch {
	case choice == spacePersonal:
		a.showSpace("")
	case strings.HasPrefix(choice, "team:"):
		a.showSpace(strings.TrimPrefix(choice, "team:"))
	case choice == "teams":
		a.page, a.settingsSection = pageSettings, sectionCloud
	}
}
