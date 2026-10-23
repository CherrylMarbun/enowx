package client

import "github.com/enowx/enowx/proto"

// Credits returns the daemon's latest reading of the agent accounts on its
// host; refresh asks it to read them again.
func (c *Client) Credits(refresh bool) (proto.CreditsReport, error) {
	var out proto.CreditsReport
	err := c.Call(proto.MethodCreditsGet, proto.CreditsParams{Refresh: refresh}, &out)
	return out, err
}
