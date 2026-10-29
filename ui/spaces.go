package ui

import (
	"encoding/json"
	"slices"

	"github.com/enowx/enowx/proto"
)

// Spaces (docs/cloud.md, Personal and team): the account's menu at the
// rail's foot (railaccount.go) says whose notes and canvases the Notes and
// Canvas pages show, the user's own or a team's. The connected host keeps each team's apart and lists them all,
// each marked with its team; the pages show the chosen space's, and what
// is made there is that space's. The menu offers them once the host is
// signed in to the cloud as a member of a team.

// hostSpaces is the connected host's teams' spaces, as last listed. It
// belongs to the main thread.
type hostSpaces struct {
	host string
	list []proto.Space
}

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
