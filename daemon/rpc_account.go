package daemon

import "github.com/enowx/enowx/proto"

var accountHandlers = map[string]handler{
	proto.MethodAccountList: bare(func(c *conn) (any, error) { return c.srv.accounts.List(), nil }),
	proto.MethodAccountAdd: with(func(c *conn, p proto.AccountAddParams) (any, error) {
		return c.srv.accounts.Add(p.Agent, p.Name)
	}),
	proto.MethodAccountRemove: with(func(c *conn, p proto.AccountRef) (any, error) {
		return ok, c.srv.accounts.Remove(p.ID)
	}),
}
