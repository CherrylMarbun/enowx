package daemon

import "github.com/enowx/enowx/proto"

// The host's sign-in to a team cloud, and requests to it as the host's
// device (cloud.go).

var cloudHandlers = map[string]handler{
	proto.MethodCloudStatus: bare(func(c *conn) (any, error) { return c.srv.cloudStatus() }),
	proto.MethodCloudSetURL: with(func(c *conn, p proto.CloudSetURLParams) (any, error) { return c.srv.cloudSetURL(p.URL) }),
	proto.MethodCloudSignIn: with(func(c *conn, p proto.CloudSignInParams) (any, error) { return c.srv.cloudSignIn(p) }),
	proto.MethodCloudCancel: bare(func(c *conn) (any, error) {
		c.srv.cloudCancel()
		return c.srv.cloudStatus()
	}),
	proto.MethodCloudSignOut: bare(func(c *conn) (any, error) { return c.srv.cloudSignOut() }),
	proto.MethodCloudRequest: with(func(c *conn, p proto.CloudRequest) (any, error) { return c.srv.cloudRequest(p) }),
	proto.MethodSpaces:       bare(func(c *conn) (any, error) { return c.srv.spaces(), nil }),
}
