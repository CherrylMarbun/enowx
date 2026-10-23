package client

import "github.com/enowx/enowx/proto"

// EnvScan lists the environment items in the daemon's home.
func (c *Client) EnvScan() (proto.EnvScan, error) {
	var out proto.EnvScan
	err := c.Call(proto.MethodEnvScan, struct{}{}, &out)
	return out, err
}

// EnvExport returns an environment item's content.
func (c *Client) EnvExport(id string) (proto.EnvPayload, error) {
	var out proto.EnvPayload
	err := c.Call(proto.MethodEnvExport, proto.EnvItemRef{ID: id}, &out)
	return out, err
}

// EnvImport writes an environment item into the daemon's home.
func (c *Client) EnvImport(p proto.EnvPayload) (proto.EnvImportResult, error) {
	var out proto.EnvImportResult
	err := c.Call(proto.MethodEnvImport, p, &out)
	return out, err
}

// EnvDelete deletes an item of the daemon's home: its files, or its entry
// in the agent's config.
func (c *Client) EnvDelete(id string) error {
	return c.Call(proto.MethodEnvDelete, proto.EnvItemRef{ID: id}, nil)
}

// EnvProjectScan lists a project's own items, under its folder.
func (c *Client) EnvProjectScan(project string) (proto.EnvScan, error) {
	var out proto.EnvScan
	err := c.Call(proto.MethodEnvProjectScan, proto.EnvProjectRef{Project: project}, &out)
	return out, err
}

// EnvProjectExport reads an item of a project.
func (c *Client) EnvProjectExport(project, id string) (proto.EnvPayload, error) {
	var out proto.EnvPayload
	err := c.Call(proto.MethodEnvProjectExport, proto.EnvProjectRef{Project: project, ID: id}, &out)
	return out, err
}

// EnvProjectImport writes an item into a project.
func (c *Client) EnvProjectImport(project string, p proto.EnvPayload) (proto.EnvImportResult, error) {
	var out proto.EnvImportResult
	err := c.Call(proto.MethodEnvProjectImport, proto.EnvProjectImport{Project: project, Payload: p}, &out)
	return out, err
}

// EnvProjectDelete deletes an item of a project.
func (c *Client) EnvProjectDelete(project, id string) error {
	return c.Call(proto.MethodEnvProjectDelete, proto.EnvProjectRef{Project: project, ID: id}, nil)
}

// EnvMCPTest runs MCP server id where its agent would (of project, or the
// user's for ""), and lists its tools.
func (c *Client) EnvMCPTest(project, id string) (proto.MCPTest, error) {
	var out proto.MCPTest
	err := c.Call(proto.MethodEnvMCPTest, proto.EnvProjectRef{Project: project, ID: id}, &out)
	return out, err
}
