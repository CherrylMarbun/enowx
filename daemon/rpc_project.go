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
	proto.MethodProjectClone: with(func(c *conn, p proto.ProjectCloneParams) (any, error) {
		return c.srv.startClone(p)
	}),
	proto.MethodProjectCloneStatus: with(func(c *conn, p proto.CloneRef) (any, error) {
		return c.srv.cloneStatus(p.ID)
	}),
	proto.MethodProjectCloneCancel: with(func(c *conn, p proto.CloneRef) (any, error) {
		return ok, c.srv.cancelClone(p.ID)
	}),
	proto.MethodProjectClones: bare(func(c *conn) (any, error) { return c.srv.cloneList(), nil }),
	proto.MethodProjectArrange: with(func(c *conn, p proto.ProjectArrangeParams) (any, error) {
		return c.srv.projectsArranged(c.srv.projects.Arrange(p.Path, p.Group, p.Before))
	}),
	proto.MethodProjectGroupMove: with(func(c *conn, p proto.ProjectGroupMoveParams) (any, error) {
		return c.srv.projectsArranged(c.srv.projects.MoveGroup(p.Group, p.Before))
	}),
	proto.MethodProjectGroupRename: with(func(c *conn, p proto.ProjectGroupRenameParams) (any, error) {
		return c.srv.projectsArranged(c.srv.projects.RenameGroup(p.From, p.To))
	}),
}

// projectsArranged tells every app the projects' new arrangement, after
// err-less changes to it.
func (s *Server) projectsArranged(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	s.broadcast(proto.EventProjectsChanged, s.projects.List())
	return ok, nil
}

var worktreeHandlers = map[string]handler{
	proto.MethodWorktreeCreate: with(func(c *conn, p proto.WorktreeCreateParams) (any, error) {
		return worktree.Create(c.srv.WorktreeRoot, p)
	}),
}
