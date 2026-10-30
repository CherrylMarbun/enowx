package daemon

import "github.com/enowx/enowx/proto"

var browserHandlers = map[string]handler{
	proto.MethodBrowserAct: with(func(c *conn, p proto.BrowserAction) (any, error) {
		return c.srv.browserAct(p)
	}),
	proto.MethodBrowserServe: bare(func(c *conn) (any, error) {
		c.srv.browsers.serve(c)
		return ok, nil
	}),
	proto.MethodBrowserReply: with(func(c *conn, p proto.BrowserReply) (any, error) {
		c.srv.browsers.reply(p)
		return ok, nil
	}),
	proto.MethodBrowserUnslop: with(func(c *conn, p proto.BrowserUnslopParams) (any, error) {
		return c.srv.browserUnslop(p)
	}),
	proto.MethodDesignReview: with(func(c *conn, p proto.DesignReviewParams) (any, error) {
		return c.srv.designReview(p)
	}),
	proto.MethodBrowserUnslopShots: with(func(c *conn, p proto.BrowserUnslopParams) (any, error) {
		return c.srv.browserUnslopShots(p.Session)
	}),
}
