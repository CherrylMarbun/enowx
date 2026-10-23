package daemon

import (
	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/worktree"
	"github.com/enowx/enowx/proto"
)

var projectHandlers = map[string]handler{
	proto.MethodProjectList: bare(func(c *conn) (any, error) { return c.srv.projects.List(), nil }),
	proto.MethodProjectAdd: with(func(c *conn, p proto.ProjectRef) (any, error) {
		info, err := c.srv.projects.Add(p.Path)
		if err != nil {
			return nil, err
		}
		c.srv.broadcast(proto.EventProjectsChanged, c.srv.projects.List())
		return info, nil
	}),
	proto.MethodProjectRemove: with(func(c *conn, p proto.ProjectRef) (any, error) {
		if err := c.srv.projects.Remove(p.Path); err != nil {
			return nil, err
		}
		c.srv.broadcast(proto.EventProjectsChanged, c.srv.projects.List())
		return ok, nil
	}),
	proto.MethodProjectInspect: with(func(c *conn, p proto.ProjectRef) (any, error) {
		return project.Inspect(p.Path), nil
	}),
}

var worktreeHandlers = map[string]handler{
	proto.MethodWorktreeCreate: with(func(c *conn, p proto.WorktreeCreateParams) (any, error) {
		return worktree.Create(c.srv.WorktreeRoot, p)
	}),
}
