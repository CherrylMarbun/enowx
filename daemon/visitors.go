package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/proto"
)

// Visitors (docs/cloud.md, Hosts and sessions): members of the host's teams
// connecting from their own devices, through the team listener. A visitor's
// connection does nothing before it hands a ticket the cloud gave them for
// this host (team.auth), which the host redeems with its own token: then
// it knows who they are and their role, and serves them what that role
// allows, no event reaching them unasked. The host's own connections (its
// socket, SSH) are its user's and unchanged.

// visitorList is who is connected as a visitor; its mutex guards conns.
type visitorList struct {
	mu    sync.Mutex
	conns map[*conn]proto.Visitor
}

// visitorMethods are what a visitor may call, by the least role each
// needs. Sharing hosts' projects opens more, each checking the project.
var visitorMethods = map[string]string{
	proto.MethodTeamAuth:       api.RoleViewer,
	proto.MethodDaemonInfo:     api.RoleViewer,
	proto.MethodProjectList:    api.RoleViewer, // the shared ones (hostshare.go)
	proto.MethodSessionList:    api.RoleViewer,
	proto.MethodProjectContext: api.RoleViewer,
	proto.MethodProjectRead:    api.RoleViewer,
}

var rank = map[string]int{api.RoleViewer: 1, api.RoleMember: 2, api.RoleAdmin: 3, api.RoleOwner: 4}

// errSignInFirst answers a visitor's connection before its ticket.
var errSignInFirst = errors.New("this host serves its teams' members: hand it a ticket first (team.auth)")

// ServeVisitorConn serves a connection from the team listener: a visitor's.
func (s *Server) ServeVisitorConn(nc net.Conn) { s.serveConn(nc, true) }

// ListenTeam serves visitors on l until it closes.
func (s *Server) ListenTeam(l net.Listener) error {
	for {
		nc, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.ServeVisitorConn(nc)
	}
}

// mayCall says whether connection c may call method.
func (c *conn) mayCall(method string) error {
	if !c.remote {
		return nil
	}
	v := c.visitor.Load()
	if v == nil {
		if method == proto.MethodTeamAuth {
			return nil
		}
		return errSignInFirst
	}
	need, ok := visitorMethods[method]
	if !ok {
		return fmt.Errorf("%s isn't open to the team's members on this host", method)
	}
	if rank[v.Role] < rank[need] {
		return fmt.Errorf("%s needs a %s of the team, you're a %s", method, need, v.Role)
	}
	return nil
}

// teamAuth redeems a visitor's ticket: who they are, from now on.
func (c *conn) teamAuth(p proto.TeamAuthParams) (proto.Visitor, error) {
	if !c.remote {
		return proto.Visitor{}, errors.New("team.auth is for visitors; this connection is the host's own")
	}
	if c.visitor.Load() != nil {
		return proto.Visitor{}, errors.New("signed in already")
	}
	s := c.srv
	st, err := s.cloud.store.Get()
	if err != nil || !st.SignedIn() {
		c.close() // nobody can be let in
		return proto.Visitor{}, errors.New("this host isn't signed in to the cloud: it lets no visitor in")
	}
	cl, err := cloudclient.New(st.URL, st.Token)
	if err != nil {
		return proto.Visitor{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloudTimeout)
	defer cancel()
	got, err := cl.RedeemTicket(ctx, strings.TrimSpace(p.Ticket))
	if err != nil {
		go c.close() // one try: a wrong ticket ends the connection
		return proto.Visitor{}, err
	}
	if _, ok := s.spaceByTeam(got.Team); !ok {
		go c.close()
		return proto.Visitor{}, errors.New("this host isn't one of that team's")
	}
	v := proto.Visitor{Account: got.Account.ID, Name: got.Account.Name, GitHub: got.Account.GitHub, Team: got.Team,
		Role: got.Role, Device: got.Device, Since: time.Now().UTC().Truncate(time.Second)}
	c.visitor.Store(&v)
	s.visitors.mu.Lock()
	if s.visitors.conns == nil {
		s.visitors.conns = map[*conn]proto.Visitor{}
	}
	s.visitors.conns[c] = v
	s.visitors.mu.Unlock()
	s.logger().Info("a team's member connected", "account", v.Account, "github", v.GitHub, "team", v.Team, "role", v.Role)
	s.visitorsChanged()
	return v, nil
}

// visitorGone forgets a visitor's connection as it closes.
func (s *Server) visitorGone(c *conn) {
	s.visitors.mu.Lock()
	v, was := s.visitors.conns[c]
	delete(s.visitors.conns, c)
	s.visitors.mu.Unlock()
	if !was {
		return
	}
	if len(s.visitorConns(v.Account)) == 0 {
		// Gone, they give back what they took over.
		for _, session := range s.takeover.of(v.Account) {
			s.takeover.set(session, "")
		}
	}
	s.visitorsChanged()
}

// hostVisitors are who is connected as a visitor, the earliest first.
func (s *Server) hostVisitors() []proto.Visitor {
	s.visitors.mu.Lock()
	conns := make(map[*conn]proto.Visitor, len(s.visitors.conns))
	for c, v := range s.visitors.conns {
		conns[c] = v
	}
	s.visitors.mu.Unlock()
	out := make([]proto.Visitor, 0, len(conns))
	for c, v := range conns {
		v.Watching, v.Controls = c.watching(), s.takeover.of(v.Account)
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b proto.Visitor) int { return a.Since.Compare(b.Since) })
	return out
}

func (s *Server) visitorsChanged() { s.broadcast(proto.EventHostVisitors, s.hostVisitors()) }

var visitorHandlers = map[string]handler{
	proto.MethodTeamAuth: with(func(c *conn, p proto.TeamAuthParams) (any, error) { return c.teamAuth(p) }),
	proto.MethodHostVisitors: bare(func(c *conn) (any, error) {
		if c.remote {
			return nil, errors.New("the host's own connections see who visits")
		}
		return c.srv.hostVisitors(), nil
	}),
}
