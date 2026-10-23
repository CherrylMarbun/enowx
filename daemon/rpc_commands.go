package daemon

import (
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

var commandHandlers = map[string]handler{
	proto.MethodCommandsList: with(func(c *conn, p proto.SessionRef) (any, error) {
		return c.srv.commandsOf(p.ID)
	}),
	proto.MethodCommandsSave: with(func(c *conn, p proto.CommandSaveParams) (any, error) {
		project, _, err := c.srv.checksProject(p.Session)
		if err != nil {
			return nil, err
		}
		if p.Remove {
			err = c.srv.commandStore.Remove(project, p.Command.Command)
		} else {
			err = c.srv.commandStore.Save(project, p.Command)
		}
		if err != nil {
			return nil, err
		}
		return c.srv.commandsOf(p.Session)
	}),
	proto.MethodCommandsRun: with(func(c *conn, p proto.CommandRunParams) (any, error) {
		return c.srv.runCommand(p)
	}),
}

// commandsOf lists the commands of session id's project.
func (s *Server) commandsOf(id string) ([]proto.RunCommand, error) {
	project, dir, err := s.checksProject(id)
	if err != nil {
		return nil, err
	}
	return s.commandStore.For(project, dir), nil
}

// runCommand types p's command into a shell in a new tab of p's session,
// so it runs as the user would run it, and they can stop it and run it
// again there; it becomes the project's latest recent command.
func (s *Server) runCommand(p proto.CommandRunParams) (proto.SessionInfo, error) {
	sess, err := s.sessions.Get(p.Session)
	if err != nil {
		return proto.SessionInfo{}, err
	}
	parent := p.Session
	if pp := sess.Meta().Parent; pp != "" {
		parent = pp
	}
	tab, err := s.createSession(proto.SessionCreateParams{
		Parent: parent, Agent: agent.Shell, Name: p.Command.Name, Prompt: p.Command.Command,
	})
	if err != nil {
		return proto.SessionInfo{}, err
	}
	if project, _, err := s.checksProject(parent); err == nil {
		_ = s.commandStore.Ran(project, p.Command)
	}
	return tab.Info(), nil
}
