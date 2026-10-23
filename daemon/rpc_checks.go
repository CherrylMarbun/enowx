package daemon

import (
	"github.com/enowx/enowx/proto"
)

var checkHandlers = map[string]handler{
	proto.MethodChecksList: with(func(c *conn, p proto.SessionRef) (any, error) {
		return c.srv.checksOf(p.ID)
	}),
	proto.MethodChecksRun: with(func(c *conn, p proto.ChecksRunParams) (any, error) {
		info, err := c.srv.checksOf(p.Session)
		if err != nil {
			return nil, err
		}
		dir, err := c.srv.sessionDir(p.Session)
		if err != nil {
			return nil, err
		}
		run := info.Checks
		if len(p.Names) > 0 {
			run = nil
			for _, ch := range info.Checks {
				for _, n := range p.Names {
					if ch.Name == n {
						run = append(run, ch)
					}
				}
			}
		}
		c.srv.runner.Run(p.Session, dir, run)
		return c.srv.checksOf(p.Session)
	}),
	proto.MethodChecksStop: with(func(c *conn, p proto.SessionRef) (any, error) {
		c.srv.runner.Stop(p.ID)
		return ok, nil
	}),
	proto.MethodChecksSet: with(func(c *conn, p proto.ChecksSetParams) (any, error) {
		project, _, err := c.srv.checksProject(p.Session)
		if err != nil {
			return nil, err
		}
		if err := c.srv.checkStore.Set(project, p.Checks); err != nil {
			return nil, err
		}
		return c.srv.checksOf(p.Session)
	}),
}

// checksProject is what session id's checks are kept by (its project, or
// its folder) and the folder they run in.
func (s *Server) checksProject(id string) (project, dir string, err error) {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return "", "", err
	}
	if dir, err = s.sessionDir(id); err != nil {
		return "", "", err
	}
	project = sess.Info().Project
	if project == "" {
		project = dir
	}
	return project, dir, nil
}

// checksOf is session id's checks and their last runs.
func (s *Server) checksOf(id string) (proto.ChecksInfo, error) {
	project, dir, err := s.checksProject(id)
	if err != nil {
		return proto.ChecksInfo{}, err
	}
	checks, custom := s.checkStore.For(project, dir)
	return proto.ChecksInfo{Checks: checks, Custom: custom, Runs: s.runner.Results(id)}, nil
}
