package client

import "github.com/enowx/enowx/proto"

// AgentAPIs are the agents of the host that take a model API of the
// user's, each with the one it is set to (its key masked).
func (c *Client) AgentAPIs() ([]proto.AgentAPIInfo, error) {
	var out []proto.AgentAPIInfo
	err := c.Call(proto.MethodAgentAPIList, nil, &out)
	return out, err
}

// SetAgentAPI points agent at api, its settings kept first.
func (c *Client) SetAgentAPI(agent string, api proto.AgentAPI) (proto.AgentAPIInfo, error) {
	var out proto.AgentAPIInfo
	err := c.Call(proto.MethodAgentAPISet, proto.AgentAPISetParams{Agent: agent, API: api}, &out)
	return out, err
}

// ClearAgentAPI sets agent back to its own API.
func (c *Client) ClearAgentAPI(agent string) (proto.AgentAPIInfo, error) {
	var out proto.AgentAPIInfo
	err := c.Call(proto.MethodAgentAPIClear, proto.AgentAPIRef{Agent: agent}, &out)
	return out, err
}

// RestoreAgentAPI puts agent's settings back as backup kept them, "" for
// as the user had them.
func (c *Client) RestoreAgentAPI(agent, backup string) (proto.AgentAPIInfo, error) {
	var out proto.AgentAPIInfo
	err := c.Call(proto.MethodAgentAPIRestore, proto.AgentAPIRestoreParams{Agent: agent, Backup: backup}, &out)
	return out, err
}
