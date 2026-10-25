package client

import "github.com/enowx/enowx/proto"

// CloneProject starts cloning a repository into a new project on the host.
func (c *Client) CloneProject(p proto.ProjectCloneParams) (proto.CloneStatus, error) {
	var st proto.CloneStatus
	err := c.Call(proto.MethodProjectClone, p, &st)
	return st, err
}

// CloneStatus is how clone id goes.
func (c *Client) CloneStatus(id string) (proto.CloneStatus, error) {
	var st proto.CloneStatus
	err := c.Call(proto.MethodProjectCloneStatus, proto.CloneRef{ID: id}, &st)
	return st, err
}

// CancelClone stops clone id, its folder removed.
func (c *Client) CancelClone(id string) error {
	return c.Call(proto.MethodProjectCloneCancel, proto.CloneRef{ID: id}, nil)
}

// Clones lists the host's clones, running and lately done.
func (c *Client) Clones() ([]proto.CloneStatus, error) {
	var list []proto.CloneStatus
	err := c.Call(proto.MethodProjectClones, nil, &list)
	return list, err
}
