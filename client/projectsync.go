package client

import "github.com/enowx/enowx/proto"

// ScanProjectSync says what the copy of the project at dir on the
// daemon's host has, to compare with another copy.
func (c *Client) ScanProjectSync(dir string, env bool) (proto.ProjectSyncState, error) {
	var st proto.ProjectSyncState
	err := c.Call(proto.MethodProjectSyncScan, proto.ProjectSyncScanParams{Project: dir, Env: env}, &st)
	return st, err
}

// PackProjectSync packs what another copy of a project lacks, to carry
// with CopyTransfer.
func (c *Client) PackProjectSync(p proto.ProjectSyncPackParams) (proto.TransferInfo, error) {
	var info proto.TransferInfo
	err := c.Call(proto.MethodProjectSyncPack, p, &info)
	return info, err
}

// ApplyProjectSync applies an archive carried from another copy.
func (c *Client) ApplyProjectSync(p proto.ProjectSyncApplyParams) (proto.ProjectSyncResult, error) {
	var res proto.ProjectSyncResult
	err := c.Call(proto.MethodProjectSyncApply, p, &res)
	return res, err
}
