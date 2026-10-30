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
