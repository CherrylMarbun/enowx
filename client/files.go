package client

import "github.com/enowx/enowx/proto"

// ListFiles lists the folder path of session id's folder ("" for itself).
func (c *Client) ListFiles(id, path string) (proto.FileListing, error) {
	var l proto.FileListing
	err := c.Call(proto.MethodFileList, proto.FileRef{Session: id, Path: path}, &l)
	return l, err
}

// ReadFile reads the file path of session id's folder.
func (c *Client) ReadFile(id, path string) (proto.FileContent, error) {
	var f proto.FileContent
	err := c.Call(proto.MethodFileRead, proto.FileRef{Session: id, Path: path}, &f)
	return f, err
}

// Ports lists the TCP ports session id's processes listen on.
func (c *Client) Ports(id string) (proto.SessionPorts, error) {
	var p proto.SessionPorts
	err := c.Call(proto.MethodSessionPorts, proto.SessionRef{ID: id}, &p)
	return p, err
}
