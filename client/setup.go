package client

import "github.com/enowx/enowx/proto"

// ScanSetup asks what the daemon's host has: toolchains, agents, git.
func (c *Client) ScanSetup() (proto.SetupScan, error) {
	var s proto.SetupScan
	err := c.Call(proto.MethodSetupScan, nil, &s)
	return s, err
}
