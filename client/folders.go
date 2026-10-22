package client

import "github.com/enowx/enowx/proto"

// ListFolders lists the folders in a folder of the daemon's host.
func (c *Client) ListFolders(path string, hidden bool) (proto.FolderListing, error) {
	var out proto.FolderListing
	err := c.Call(proto.MethodFolderList, proto.FolderListParams{Path: path, Hidden: hidden}, &out)
	return out, err
}

// MakeFolder creates a folder on the daemon's host and returns its path.
func (c *Client) MakeFolder(path string) (string, error) {
	var out proto.FolderRef
	err := c.Call(proto.MethodFolderMake, proto.FolderRef{Path: path}, &out)
	return out.Path, err
}
