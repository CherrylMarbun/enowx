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
