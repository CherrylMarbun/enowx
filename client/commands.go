package client

import "github.com/enowx/enowx/proto"

// ProjectCommands lists the commands of session id's project: saved, run lately,
// then found in its files.
func (c *Client) ProjectCommands(id string) ([]proto.RunCommand, error) {
	var out []proto.RunCommand
	err := c.Call(proto.MethodCommandsList, proto.SessionRef{ID: id}, &out)
	return out, err
}

// RunCommand runs cmd in a new tab of session id, typed into a shell.
func (c *Client) RunCommand(id string, cmd proto.RunCommand) (proto.SessionInfo, error) {
	var info proto.SessionInfo
	err := c.Call(proto.MethodCommandsRun, proto.CommandRunParams{Session: id, Command: cmd}, &info)
	return info, err
}

// SaveCommand saves cmd for session id's project, or forgets it with
// remove; it answers the project's commands.
func (c *Client) SaveCommand(id string, cmd proto.RunCommand, remove bool) ([]proto.RunCommand, error) {
	var out []proto.RunCommand
	err := c.Call(proto.MethodCommandsSave, proto.CommandSaveParams{Session: id, Command: cmd, Remove: remove}, &out)
	return out, err
}
