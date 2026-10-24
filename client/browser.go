package client

import "github.com/enowx/enowx/proto"

// BrowserAct has the app showing a session's Browser do a (enowx browser).
func (c *Client) BrowserAct(a proto.BrowserAction) (proto.BrowserResult, error) {
	var out proto.BrowserResult
	err := c.Call(proto.MethodBrowserAct, a, &out)
	return out, err
}

// ServeBrowsers tells the daemon this app does browser.act's actions,
// which come as proto.EventBrowserRequest events.
func (c *Client) ServeBrowsers() error { return c.Call(proto.MethodBrowserServe, nil, nil) }

// BrowserReply answers a proto.EventBrowserRequest.
func (c *Client) BrowserReply(r proto.BrowserReply) error {
	return c.Call(proto.MethodBrowserReply, r, nil)
}
