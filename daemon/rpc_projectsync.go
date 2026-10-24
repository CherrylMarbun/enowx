package daemon

import "github.com/enowx/enowx/proto"

var projectSyncHandlers = map[string]handler{
	proto.MethodProjectSyncScan: with(func(c *conn, p proto.ProjectSyncScanParams) (any, error) {
		return c.srv.syncScan(p)
	}),
	proto.MethodProjectSyncPack: with(func(c *conn, p proto.ProjectSyncPackParams) (any, error) {
		return c.srv.syncPack(p)
	}),
	proto.MethodProjectSyncApply: with(func(c *conn, p proto.ProjectSyncApplyParams) (any, error) {
		return c.srv.syncApply(p)
	}),
}
