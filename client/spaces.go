package client

import "github.com/enowx/enowx/proto"

// Spaces lists the teams' spaces on the host; none from a daemon of
// before, or while it isn't signed in to the cloud.
func (c *Client) Spaces() ([]proto.Space, error) {
	var list []proto.Space
	err := c.Call(proto.MethodSpaces, nil, &list)
	if IsUnknownMethod(err) {
		return nil, nil
	}
	return list, err
}

// SyncSpace syncs team's space with the cloud now, and says how it went.
func (c *Client) SyncSpace(team string) (proto.Space, error) {
	var sp proto.Space
	err := c.Call(proto.MethodSpaceSync, proto.SpaceRef{Team: team}, &sp)
	return sp, err
}

// TeamProjects lists every team's projects, each with its copies on the
// host; none from a daemon of before.
func (c *Client) TeamProjects() ([]proto.TeamProject, error) {
	var list []proto.TeamProject
	err := c.Call(proto.MethodTeamProjects, nil, &list)
	if IsUnknownMethod(err) {
		return nil, nil
	}
	return list, err
}

// AddTeamProject adds a Git link to a team, as one of its admins.
func (c *Client) AddTeamProject(p proto.TeamProjectAddParams) (proto.TeamProject, error) {
	var tp proto.TeamProject
	err := c.Call(proto.MethodTeamProjectAdd, p, &tp)
	return tp, err
}

// RemoveTeamProject takes a project out of a team, as one of its admins.
func (c *Client) RemoveTeamProject(team, id string) error {
	return c.Call(proto.MethodTeamProjectRemove, proto.TeamProjectRef{Team: team, ID: id}, nil)
}

// MoveNoteToSpace takes a note, and the pages under it, to team's space
// ("" the user's own), or copies them there.
func (c *Client) MoveNoteToSpace(id, team string, copy bool) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteMoveSpace, proto.SpaceMoveParams{ID: id, Team: team, Copy: copy}, &info)
	return info, err
}

// MoveCanvasToSpace takes a canvas to team's space, or copies it there.
func (c *Client) MoveCanvasToSpace(id, team string, copy bool) (proto.CanvasInfo, error) {
	var info proto.CanvasInfo
	err := c.Call(proto.MethodCanvasMoveSpace, proto.SpaceMoveParams{ID: id, Team: team, Copy: copy}, &info)
	return info, err
}

// TeamKit lists every team's kit.
func (c *Client) TeamKit() ([]proto.TeamKitItem, error) {
	var list []proto.TeamKitItem
	err := c.Call(proto.MethodTeamKit, nil, &list)
	if IsUnknownMethod(err) {
		return nil, nil
	}
	return list, err
}

// AddToTeamKit copies the host's agent item (an EnvItem's ID) into team's
// kit.
func (c *Client) AddToTeamKit(team, item string) (proto.TeamKitItem, error) {
	var it proto.TeamKitItem
	err := c.Call(proto.MethodTeamKitAdd, proto.TeamKitAddParams{Team: team, Item: item}, &it)
	return it, err
}

// RemoveFromTeamKit takes an item out of team's kit.
func (c *Client) RemoveFromTeamKit(team, kind, name string) error {
	return c.Call(proto.MethodTeamKitRemove, proto.TeamKitRef{Team: team, Kind: kind, Name: name}, nil)
}

// ProjectRules reads the rules of the project at folder path.
func (c *Client) ProjectRules(path string) (proto.ProjectRules, error) {
	var r proto.ProjectRules
	err := c.Call(proto.MethodProjectRules, proto.ProjectRulesRef{Path: path}, &r)
	return r, err
}

// SaveProjectRules writes a project's rules (and, a team's admins, who
// edit them).
func (c *Client) SaveProjectRules(r proto.ProjectRules) (proto.ProjectRules, error) {
	var out proto.ProjectRules
	err := c.Call(proto.MethodProjectRulesSave, r, &out)
	return out, err
}

// SessionsEverywhere lists the sessions of the host and of every host
// the app reaches.
func (c *Client) SessionsEverywhere() ([]proto.HostSessions, error) {
	var out []proto.HostSessions
	err := c.Call(proto.MethodSessionsEverywhere, nil, &out)
	return out, err
}

// PeekSession reads a session's conversation, on the host or another.
func (c *Client) PeekSession(p proto.SessionPeekParams) (proto.SessionPeek, error) {
	var out proto.SessionPeek
	err := c.Call(proto.MethodSessionPeek, p, &out)
	return out, err
}
