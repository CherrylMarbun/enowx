package client

import "github.com/enowx/enowx/proto"

// Checks returns session id's checks and how they last went.
func (c *Client) Checks(id string) (proto.ChecksInfo, error) {
	var info proto.ChecksInfo
	err := c.Call(proto.MethodChecksList, proto.SessionRef{ID: id}, &info)
	return info, err
}

// RunChecks runs session id's checks named, or all of them.
func (c *Client) RunChecks(id string, names ...string) (proto.ChecksInfo, error) {
	var info proto.ChecksInfo
	err := c.Call(proto.MethodChecksRun, proto.ChecksRunParams{Session: id, Names: names}, &info)
	return info, err
}

// StopChecks stops session id's checks under way.
func (c *Client) StopChecks(id string) error {
	return c.Call(proto.MethodChecksStop, proto.SessionRef{ID: id}, nil)
}

// SetChecks sets the checks of session id's project; none goes back to
// those found from its files.
func (c *Client) SetChecks(id string, checks []proto.Check) (proto.ChecksInfo, error) {
	var info proto.ChecksInfo
	err := c.Call(proto.MethodChecksSet, proto.ChecksSetParams{Session: id, Checks: checks}, &info)
	return info, err
}
