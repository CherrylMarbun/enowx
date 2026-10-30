package daemon

import (
	"github.com/enowx/enowx/proto"
)

// hookHandlers keep the host's hooks and take the events agents' own
// hooks hand over (daemon/hooks.go).
var hookHandlers = map[string]handler{
	proto.MethodHooksList: with(func(c *conn, p proto.ProjectDir) (any, error) { return c.srv.hooksInfo(p.Dir), nil }),
	proto.MethodHooksSave: with(func(c *conn, p []proto.Hook) (any, error) {
		return c.srv.hooks.store.Save(p)
	}),
	proto.MethodHooksApprove: with(func(c *conn, p proto.HooksApproval) (any, error) {
		return ok, c.srv.approveHooks(p)
	}),
	proto.MethodHooksTest: with(func(c *conn, p proto.HookTest) (any, error) { return c.srv.testHook(p) }),
	proto.MethodHooksLog:  bare(func(c *conn) (any, error) { return c.srv.hookLog(), nil }),
	proto.MethodSessionHook: with(func(c *conn, p proto.SessionHookParams) (any, error) {
		return c.srv.sessionHook(p)
	}),
}

// sessionHook takes event p from a session's agent: the state it means,
// the turn's start or end, then the user's hooks and what they decided.
func (s *Server) sessionHook(p proto.SessionHookParams) (proto.HookDecision, error) {
	sess, err := s.sessions.Get(p.ID)
	if err != nil {
		return proto.HookDecision{}, err
	}
	if p.State != "" {
		if err := s.sessions.SetState(sess, p.State); err != nil {
			return proto.HookDecision{}, err
		}
	}
	info := sess.Info()
	if info.NotesFor != "" || info.CanvasFor != "" {
		return proto.HookDecision{}, nil // the app's own agents
	}
	ev := p.Event
	ev.Via = proto.HookInAgent
	d := s.fireHooks(info, ev)
	switch ev.Event {
	case proto.HookPromptStart:
		if !d.Block {
			go s.turnStarted(info)
		}
	case proto.HookTurnDone:
		go s.turnEnded(info)
	}
	return d, nil
}
