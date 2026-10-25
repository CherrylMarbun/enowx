package client

import (
	"encoding/json"

	"github.com/enowx/enowx/proto"
)

// ScanProjectSyncWith says what a copy of a project has, by p's rules.
func (c *Client) ScanProjectSyncWith(p proto.ProjectSyncScanParams) (proto.ProjectSyncState, error) {
	var st proto.ProjectSyncState
	err := c.Call(proto.MethodProjectSyncScan, p, &st)
	return st, err
}

// ProjectID is the ID of the project at dir, made when it has none.
func (c *Client) ProjectID(dir string) (proto.ProjectIdentity, error) {
	var id proto.ProjectIdentity
	err := c.Call(proto.MethodProjectID, proto.ProjectRef{Path: dir}, &id)
	return id, err
}

// FindProject is the project on the host with ID id: the folder dir when
// it still is, else the first of the host's projects that is.
func (c *Client) FindProject(id, dir string) (proto.ProjectInfo, error) {
	var info proto.ProjectInfo
	err := c.Call(proto.MethodProjectFind, proto.ProjectIdentity{ID: id, Path: dir}, &info)
	return info, err
}

// ProjectLinks lists the links of the project with ID project (all with
// ""), with their status as this host was told.
func (c *Client) ProjectLinks(project string) ([]proto.ProjectLinkState, error) {
	var list []proto.ProjectLinkState
	err := c.Call(proto.MethodProjectLinks, proto.ProjectLinksParams{Project: project}, &list)
	return list, err
}

// SaveProjectLink records a link: a new one without ID.
func (c *Client) SaveProjectLink(l proto.ProjectLink) (proto.ProjectLink, error) {
	var out proto.ProjectLink
	err := c.Call(proto.MethodProjectLinkSave, l, &out)
	return out, err
}

// DeleteProjectLink forgets a link; both copies stay.
func (c *Client) DeleteProjectLink(id string) error {
	return c.Call(proto.MethodProjectLinkDelete, proto.ProjectLinkRef{ID: id}, nil)
}

// ReportProjectLink records how a sync of a link went.
func (c *Client) ReportProjectLink(r proto.ProjectLinkReport) error {
	return c.Call(proto.MethodProjectLinkReport, r, nil)
}

// ProjectLinkBase is what a link's copies had alike at its last sync, as
// this host was told (a projectsync.Base in JSON), nil for none.
func (c *Client) ProjectLinkBase(id string) (json.RawMessage, error) {
	var b json.RawMessage
	err := c.Call(proto.MethodProjectLinkBase, proto.ProjectLinkRef{ID: id}, &b)
	if string(b) == "null" {
		b = nil
	}
	return b, err
}

// ProjectHistory lists the syncs and pushes of the project with ID
// project (all with ""), the latest first.
func (c *Client) ProjectHistory(project string) ([]proto.ProjectHistoryEntry, error) {
	var list []proto.ProjectHistoryEntry
	err := c.Call(proto.MethodProjectHistory, proto.ProjectLinksParams{Project: project}, &list)
	return list, err
}

// WatchProject has the host tell when the project at dir's files change
// (EventProjectFilesChanged), or stop with off.
func (c *Client) WatchProject(dir string, off bool) error {
	return c.Call(proto.MethodProjectWatch, proto.ProjectWatchParams{Project: dir, Off: off}, nil)
}

// CopyProjectToHost has the app copy a project on this host to another.
func (c *Client) CopyProjectToHost(p proto.ProjectCopyParams) (proto.ProjectCopyResult, error) {
	var res proto.ProjectCopyResult
	err := c.Call(proto.MethodProjectCopyRequest, p, &res)
	return res, err
}

// SyncProjectLinkNow has the app sync a link now.
func (c *Client) SyncProjectLinkNow(id string) (proto.ProjectLinkState, error) {
	var st proto.ProjectLinkState
	err := c.Call(proto.MethodProjectSyncRequest, proto.ProjectLinkRef{ID: id}, &st)
	return st, err
}

// ServeApp tells the daemon this connection is the app's, which takes
// EventAppRequest.
func (c *Client) ServeApp() error { return c.Call(proto.MethodAppServe, nil, nil) }

// ProjectFile reads a file of the project at dir, to compare versions.
func (c *Client) ProjectFile(dir, path string) ([]byte, error) {
	var data []byte
	err := c.Call(proto.MethodProjectFileRead, proto.ProjectFileParams{Project: dir, Path: path}, &data)
	return data, err
}

// ReplyToApp answers an EventAppRequest.
func (c *Client) ReplyToApp(r proto.AppReply) error {
	return c.Call(proto.MethodAppReply, r, nil)
}

// KeepProjectCopy copies a project's file beside it, marked with suffix,
// and returns the copy's path.
func (c *Client) KeepProjectCopy(p proto.ProjectKeepCopyParams) (string, error) {
	var rel string
	err := c.Call(proto.MethodProjectKeepCopy, p, &rel)
	return rel, err
}

// Autopushes lists the projects' auto pushes.
func (c *Client) Autopushes() ([]proto.AutopushStatus, error) {
	var list []proto.AutopushStatus
	err := c.Call(proto.MethodAutopushList, nil, &list)
	return list, err
}

// SetAutopush records a project's auto push settings.
func (c *Client) SetAutopush(a proto.AutopushSettings) (proto.AutopushStatus, error) {
	var st proto.AutopushStatus
	err := c.Call(proto.MethodAutopushSet, a, &st)
	return st, err
}

// AutopushNow pushes the project at dir now.
func (c *Client) AutopushNow(dir string) (proto.AutopushStatus, error) {
	var st proto.AutopushStatus
	err := c.Call(proto.MethodAutopushNow, proto.ProjectRef{Path: dir}, &st)
	return st, err
}
