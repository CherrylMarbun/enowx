package daemon

import (
	"fmt"
	"time"

	"github.com/enowx/enowx/proto"
)

var sessionHandlers = map[string]handler{
	proto.MethodSessionCreate: with(func(c *conn, p proto.SessionCreateParams) (any, error) {
		sess, err := c.srv.createSession(p)
		if err != nil {
			return nil, err
		}
		return sess.Info(), nil
	}),
	proto.MethodSessionList: bare(func(c *conn) (any, error) {
		list := c.srv.sessions.List()
		infos := make([]proto.SessionInfo, 0, len(list))
		for _, sess := range list {
			infos = append(infos, sess.Info())
		}
		return infos, nil
	}),
	proto.MethodSessionAttach: with(func(c *conn, p proto.SessionRef) (any, error) {
		id, err := c.attach(p.ID)
		if err != nil {
			return nil, err
		}
		return proto.AttachResult{Stream: id}, nil
	}),
	proto.MethodSessionDetach: with(func(c *conn, p proto.StreamRef) (any, error) {
		c.detach(p.Stream)
		return ok, nil
	}),
	proto.MethodSessionResize: with(func(c *conn, p proto.ResizeParams) (any, error) {
		st := c.stream(p.Stream)
		if st == nil {
			return nil, fmt.Errorf("unknown stream %d", p.Stream)
		}
		st.resize(p)
		return ok, nil
	}),
	proto.MethodSessionTranscript: with(func(c *conn, p proto.TranscriptParams) (any, error) {
		return c.srv.transcript(p)
	}),
	proto.MethodSessionMode: with(func(c *conn, p proto.SessionModeParams) (any, error) {
		return ok, c.srv.switchMode(p)
	}),
	proto.MethodSessionCommands: with(func(c *conn, p proto.SessionRef) (any, error) {
		return c.srv.commands(p)
	}),
	proto.MethodSessionState: with(func(c *conn, p proto.SessionStateParams) (any, error) {
		sess, err := c.srv.sessions.Get(p.ID)
		if err != nil {
			return nil, err
		}
		return ok, c.srv.sessions.SetState(sess, p.State)
	}),
	proto.MethodSessionBudget: with(func(c *conn, p proto.SessionBudgetParams) (any, error) {
		sess, err := c.srv.sessions.Get(p.ID)
		if err != nil {
			return nil, err
		}
		c.srv.sessions.SetBudget(sess, time.Duration(p.Seconds)*time.Second)
		c.srv.persist.update(sess, nil)
		return ok, nil
	}),
	proto.MethodSessionPorts: with(func(c *conn, p proto.SessionRef) (any, error) {
		return c.srv.ports(p.ID)
	}),
	proto.MethodSessionClear: with(func(c *conn, p proto.SessionRef) (any, error) {
		sess, err := c.srv.sessions.Get(p.ID)
		if err != nil {
			return nil, err
		}
		sess.Terminal().ClearScreen()
		return ok, nil
	}),
	proto.MethodSessionKill: with(func(c *conn, p proto.SessionRef) (any, error) {
		if err := c.srv.deleteSession(p.ID); err != nil {
			return nil, err
		}
		c.srv.broadcast(proto.EventSessionRemoved, p)
		return ok, nil
	}),
}
