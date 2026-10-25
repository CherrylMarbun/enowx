package daemon

import "github.com/enowx/enowx/proto"

var projectLinkHandlers = map[string]handler{
	proto.MethodProjectID: with(func(c *conn, p proto.ProjectRef) (any, error) {
		return c.srv.projectID(p.Path)
	}),
	proto.MethodProjectFind: with(func(c *conn, p proto.ProjectIdentity) (any, error) {
		return c.srv.findProject(p)
	}),
	proto.MethodProjectLinks: with(func(c *conn, p proto.ProjectLinksParams) (any, error) {
		return c.srv.links.States(p.Project)
	}),
	proto.MethodProjectLinkSave: with(func(c *conn, p proto.ProjectLink) (any, error) {
		return c.srv.saveLink(p)
	}),
	proto.MethodProjectLinkDelete: with(func(c *conn, p proto.ProjectLinkRef) (any, error) {
		if err := c.srv.links.DeleteLink(p.ID); err != nil {
			return nil, err
		}
		c.srv.linksChanged()
		return ok, nil
	}),
	proto.MethodProjectLinkReport: with(func(c *conn, p proto.ProjectLinkReport) (any, error) {
		return ok, c.srv.reportLink(p)
	}),
	proto.MethodProjectHistory: with(func(c *conn, p proto.ProjectLinksParams) (any, error) {
		return c.srv.links.History(p.Project)
	}),
	proto.MethodProjectWatch: with(func(c *conn, p proto.ProjectWatchParams) (any, error) {
		return ok, c.srv.watchProject(c, p)
	}),
	proto.MethodProjectCopyRequest: with(func(c *conn, p proto.ProjectCopyParams) (any, error) {
		return c.srv.copyProject(p)
	}),
	proto.MethodProjectSyncRequest: with(func(c *conn, p proto.ProjectLinkRef) (any, error) {
		return c.srv.syncLinkNow(p.ID)
	}),
	proto.MethodAppReply: with(func(c *conn, p proto.AppReply) (any, error) {
		c.srv.appReqs.reply(p)
		return ok, nil
	}),
	proto.MethodProjectKeepCopy: with(func(c *conn, p proto.ProjectKeepCopyParams) (any, error) {
		return c.srv.keepCopy(p)
	}),
	proto.MethodAppServe: bare(func(c *conn) (any, error) {
		c.srv.appReqs.serve(c)
		return ok, nil
	}),
	proto.MethodProjectFileRead: with(func(c *conn, p proto.ProjectFileParams) (any, error) {
		return c.srv.projectFile(p)
	}),
	proto.MethodProjectLinkBase: with(func(c *conn, p proto.ProjectLinkRef) (any, error) {
		return c.srv.links.Base(p.ID), nil
	}),
	proto.MethodAutopushList: bare(func(c *conn) (any, error) {
		return c.srv.autopushList()
	}),
	proto.MethodAutopushSet: with(func(c *conn, p proto.AutopushSettings) (any, error) {
		return c.srv.autopushSet(p)
	}),
	proto.MethodAutopushNow: with(func(c *conn, p proto.ProjectRef) (any, error) {
		return c.srv.autopushNow(p.Path)
	}),
}
