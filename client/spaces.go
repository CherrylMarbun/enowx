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
