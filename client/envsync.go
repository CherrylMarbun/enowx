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
