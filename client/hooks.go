package client

import "github.com/enowx/enowx/proto"

// Hooks lists the host's hooks, those of the project in dir ("" for
// none) and the project hook files waiting for the user.
func (c *Client) Hooks(dir string) (proto.HooksInfo, error) {
	var out proto.HooksInfo
	err := c.Call(proto.MethodHooksList, proto.ProjectDir{Dir: dir}, &out)
	return out, err
}

// SaveHooks replaces the host's own hooks with list, returned with their
// IDs.
func (c *Client) SaveHooks(list []proto.Hook) ([]proto.Hook, error) {
	var out []proto.Hook
	err := c.Call(proto.MethodHooksSave, list, &out)
	return out, err
}

// ApproveHooks allows, or stops allowing, a project's hook file.
func (c *Client) ApproveHooks(p proto.HooksApproval) error {
	return c.Call(proto.MethodHooksApprove, p, nil)
}

// TestHook runs a hook now on a sample event.
func (c *Client) TestHook(t proto.HookTest) (proto.HookRun, error) {
	var out proto.HookRun
	err := c.Call(proto.MethodHooksTest, t, &out)
	return out, err
}

// HookLog is the host's latest hook runs, the latest first.
func (c *Client) HookLog() ([]proto.HookRun, error) {
	var out []proto.HookRun
	err := c.Call(proto.MethodHooksLog, nil, &out)
	return out, err
}

// SaveProjectHooks writes the hooks of the project in dir, allowed as
// written.
func (c *Client) SaveProjectHooks(dir string, list []proto.Hook) error {
	return c.Call(proto.MethodHooksSaveProject, proto.ProjectHooks{Dir: dir, Hooks: list}, nil)
}

// SaveSessionHooks replaces a session's own hooks.
func (c *Client) SaveSessionHooks(id string, list []proto.Hook) error {
	return c.Call(proto.MethodHooksSaveSession, proto.SessionHooks{Session: id, Hooks: list}, nil)
}

// ProposeHook proposes a change to the hooks, for the user to decide.
func (c *Client) ProposeHook(p proto.HookProposal) (proto.HookProposal, error) {
	var out proto.HookProposal
	err := c.Call(proto.MethodHooksPropose, p, &out)
	return out, err
}

// DecideHook allows or rejects a proposal.
func (c *Client) DecideHook(id string, allow bool) error {
	return c.Call(proto.MethodHooksDecide, proto.HookDecideParams{ID: id, Allow: allow}, nil)
}
