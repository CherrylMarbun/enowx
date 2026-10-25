package daemon

import "github.com/enowx/enowx/proto"

var hibernateHandlers = map[string]handler{
	proto.MethodSessionHibernate: with(func(c *conn, p proto.SessionHibernateParams) (any, error) {
		return c.srv.hibernateSessions(p)
	}),
	proto.MethodSessionWake: with(func(c *conn, p proto.SessionRef) (any, error) {
		sess, err := c.srv.continueSession(p.ID)
		if err != nil {
			return nil, err
		}
		return sess.Info(), nil
	}),
	proto.MethodSessionAutoHibernate: with(func(c *conn, p proto.SessionAutoHibernateParams) (any, error) {
		return ok, c.srv.setAutoHibernate(p)
	}),
}
