package cli

import (
	"os"
	"slices"

	"github.com/enowx/enowx/proto"
)

// In a team's project (docs/cloud.md) a session's agent works with the
// team's notes and canvases: the user's own notes show only when attached
// to the project, with the pages under them, and the user's own canvases
// not at all.

// sessionTeam is the team whose project the session runs in, and the
// project's ref; "" outside one.
func sessionTeam() (team, ref string) {
	return os.Getenv(proto.EnvTeam), os.Getenv(proto.EnvTeamProject)
}

// notesSeen are the host's notes this session sees.
func (r runner) notesSeen() ([]proto.NoteInfo, error) {
	list, err := r.c.Notes()
	if err != nil {
		return nil, err
	}
	team, ref := sessionTeam()
	if team == "" {
		return list, nil
	}
	seen := map[string]bool{}
	for _, n := range list {
		if n.Team == team || n.Team == "" && ref != "" && slices.ContainsFunc(n.Projects, func(p proto.NoteProject) bool { return p.Ref == ref }) {
			seen[n.ID] = true
		}
	}
	// The pages under those seen, in the same space.
	for grew := true; grew; {
		grew = false
		for _, n := range list {
			if !seen[n.ID] && n.Parent != "" && seen[n.Parent] {
				seen[n.ID], grew = true, true
			}
		}
	}
	return slices.DeleteFunc(list, func(n proto.NoteInfo) bool { return !seen[n.ID] }), nil
}

// groupsSeen are the groups of notes this session sees: the team's, and
// the user's own holding a note it sees.
func (r runner) groupsSeen(notes []proto.NoteInfo) []proto.NoteGroup {
	groups, _ := r.c.NoteGroups() // none from a daemon of before
	team, _ := sessionTeam()
	if team == "" {
		return groups
	}
	return slices.DeleteFunc(groups, func(g proto.NoteGroup) bool {
		return g.Team != team && !slices.ContainsFunc(notes, func(n proto.NoteInfo) bool { return n.Team == g.Team && n.Group == g.Name })
	})
}

// canvasesSeen are the host's canvases this session sees.
func (r runner) canvasesSeen() ([]proto.CanvasInfo, error) {
	list, err := r.c.Canvases()
	if err != nil {
		return nil, err
	}
	if team, _ := sessionTeam(); team != "" {
		list = slices.DeleteFunc(list, func(c proto.CanvasInfo) bool { return c.Team != team })
	}
	return list, nil
}
