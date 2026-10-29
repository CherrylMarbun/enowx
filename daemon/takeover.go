package daemon

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
)

// Watching and taking over a shared session (docs/cloud.md, Hosts and
// sessions): a visitor attaches to a session of a project shared with
// their team and sees it live, read-only: what they type and their window's
// size reach it only once the host's user lets them take over, which the
// user takes back when they will. The host's own connections see who
// watches what, and who has control.

// takeovers are the sessions visitors may type into; its mutex guards
// grants.
type takeovers struct {
	mu     sync.Mutex
	grants map[string]string // session → the visitor's account
}

func (t *takeovers) controls(session, account string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return account != "" && t.grants[session] == account
}

func (t *takeovers) set(session, account string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.grants == nil {
		t.grants = map[string]string{}
	}
	if account == "" {
		delete(t.grants, session)
	} else {
		t.grants[session] = account
	}
}

// of are the sessions account controls.
func (t *takeovers) of(account string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []string
	for s, a := range t.grants {
		if a == account {
			out = append(out, s)
		}
	}
	slices.Sort(out)
	return out
}

// mayType says what this connection types into session reaches it: the
// host's own always, a visitor's once they took over.
func (c *conn) mayType(session string) bool {
	if !c.remote {
		return true
	}
	v := c.visitor.Load()
	return v != nil && (c.srv.takeover.controls(session, v.Account) || c.srv.ownsSession(session, v.Account))
}

// sessionInScope says session id is in a project shared with the
// visitor's team.
func (c *conn) sessionInScope(id string) bool {
	sess, err := c.srv.sessions.Get(id)
	if err != nil {
		return false
	}
	info := sess.Info()
	scope := c.visitorScope()
	return info.NotesFor == "" && info.CanvasFor == "" && (inScope(scope, info.Project) || inScope(scope, info.Dir))
}

// watching are the sessions connection c has open.
func (c *conn) watching() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, st := range c.streams {
		if id := st.sess.ID(); !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// visitorConns are the connections of visitor account.
func (s *Server) visitorConns(account string) []*conn {
	s.visitors.mu.Lock()
	defer s.visitors.mu.Unlock()
	var out []*conn
	for c, v := range s.visitors.conns {
		if v.Account == account {
			out = append(out, c)
		}
	}
	return out
}

// ownBroadcast sends an event to the host's own connections alone, as
// broadcast does.
func (s *Server) ownBroadcast(name string, data any) { s.broadcast(name, data) }

var errNotInScope = errors.New("that session isn't in a project shared with your team")

// call runs the host's own handler of method for c.
func call(c *conn, method string, raw json.RawMessage) (any, error) {
	h, ok := handlers[method]
	if !ok {
		return nil, errors.New("unknown method " + method)
	}
	return h(c, raw)
}

func init() {
	// A visitor's terminal, their transcript: the shared sessions alone,
	// typing and resizing only once they took over.
	visitorOverrides[proto.MethodSessionAttach] = func(c *conn, raw json.RawMessage) (any, error) {
		var p proto.SessionRef
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if !c.sessionInScope(p.ID) {
			return nil, errNotInScope
		}
		out, err := call(c, proto.MethodSessionAttach, raw)
		c.srv.visitorsChanged()
		return out, err
	}
	visitorOverrides[proto.MethodSessionDetach] = func(c *conn, raw json.RawMessage) (any, error) {
		out, err := call(c, proto.MethodSessionDetach, raw)
		c.srv.visitorsChanged()
		return out, err
	}
	visitorOverrides[proto.MethodSessionResize] = func(c *conn, raw json.RawMessage) (any, error) {
		var p proto.ResizeParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if st := c.stream(p.Stream); st == nil || !c.mayType(st.sess.ID()) {
			return ok, nil // watching: the window keeps the owner's size
		}
		return call(c, proto.MethodSessionResize, raw)
	}
	visitorOverrides[proto.MethodSessionTranscript] = func(c *conn, raw json.RawMessage) (any, error) {
		var p proto.TranscriptParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		if !c.sessionInScope(p.ID) {
			return nil, errNotInScope
		}
		return call(c, proto.MethodSessionTranscript, raw)
	}
	for m, role := range map[string]string{proto.MethodSessionAttach: api.RoleViewer, proto.MethodSessionDetach: api.RoleViewer,
		proto.MethodSessionResize: api.RoleViewer, proto.MethodSessionTranscript: api.RoleViewer,
		proto.MethodTakeoverAsk: api.RoleMember, proto.MethodTakeoverEnd: api.RoleViewer} {
		visitorMethods[m] = role
	}
	for m, h := range takeoverHandlers {
		handlers[m] = h
	}
}

var takeoverHandlers = map[string]handler{
	proto.MethodTakeoverAsk: with(func(c *conn, p proto.SessionRef) (any, error) {
		v := c.visitor.Load()
		if v == nil {
			return nil, errors.New("only a visitor asks to take over")
		}
		if !c.sessionInScope(p.ID) {
			return nil, errNotInScope
		}
		sess, _ := c.srv.sessions.Get(p.ID)
		c.srv.ownBroadcast(proto.EventTakeoverAsked, proto.TakeoverAsk{Session: p.ID, Name: sess.Info().Name, Visitor: *v})
		return ok, nil
	}),
	proto.MethodTakeoverAnswer: with(func(c *conn, p proto.TakeoverAnswer) (any, error) {
		if c.remote {
			return nil, errors.New("the host's user answers")
		}
		if _, err := c.srv.sessions.Get(p.Session); err != nil {
			return nil, err
		}
		if p.Allow {
			c.srv.takeover.set(p.Session, p.Account)
			c.srv.logger().Info("a team's member took over a session", "session", p.Session, "account", p.Account)
		}
		for _, vc := range c.srv.visitorConns(p.Account) {
			vc.event(proto.EventTakeoverAnswered, p)
		}
		c.srv.visitorsChanged()
		return ok, nil
	}),
	proto.MethodTakeoverEnd: with(func(c *conn, p proto.TakeoverAnswer) (any, error) {
		account := p.Account
		if v := c.visitor.Load(); v != nil {
			account = v.Account // a visitor gives back their own
		} else if c.remote {
			return nil, errSignInFirst
		}
		if !c.srv.takeover.controls(p.Session, account) {
			return ok, nil
		}
		c.srv.takeover.set(p.Session, "")
		end := proto.TakeoverAnswer{Session: p.Session, Account: account}
		for _, vc := range c.srv.visitorConns(account) {
			vc.event(proto.EventTakeoverAnswered, end)
		}
		c.srv.visitorsChanged()
		return ok, nil
	}),
}
