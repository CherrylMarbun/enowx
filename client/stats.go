package client

import "github.com/enowx/enowx/proto"

// Stats measures the daemon's host and sessions.
func (c *Client) Stats() (proto.DaemonStats, error) {
	var out proto.DaemonStats
	err := c.Call(proto.MethodDaemonStats, struct{}{}, &out)
	return out, err
}
