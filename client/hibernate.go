package client

import "github.com/enowx/enowx/proto"

// HibernateSessions hibernates sessions by hand, or plans it (DryRun).
func (c *Client) HibernateSessions(p proto.SessionHibernateParams) (proto.SessionHibernateResult, error) {
	var out proto.SessionHibernateResult
	err := c.Call(proto.MethodSessionHibernate, p, &out)
	return out, err
}

// WakeSession wakes a hibernated session on its conversation.
func (c *Client) WakeSession(id string) (proto.SessionInfo, error) {
	var info proto.SessionInfo
	err := c.Call(proto.MethodSessionWake, proto.SessionRef{ID: id}, &info)
	return info, err
}

// SetAutoHibernate keeps a session (or, with Default, new sessions) from
// hibernating on its own, or lets it again.
func (c *Client) SetAutoHibernate(p proto.SessionAutoHibernateParams) error {
	return c.Call(proto.MethodSessionAutoHibernate, p, nil)
}
