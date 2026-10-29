package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/core/projectctx"
	"github.com/enowx/enowx/core/tailnet"
	"github.com/enowx/enowx/proto"
)

// A host shared with its teams (docs/cloud.md, Hosts and sessions): the
// projects its user picks for each team, in share.json. While any is
// shared, the team listener serves the teams' members on the host's
// Tailscale address, and the host announces itself in each team's space
// (hosts/<its device>.json, sealed and synced with it), so the members'
// hosts list it. A visitor sees the shared projects of their team alone.

// listenTeam serves visitors on a listener: ListenTeam, set in init, as
// the handlers it reaches lead back here.
var listenTeam func(*Server, net.Listener) error

func init() { listenTeam = (*Server).ListenTeam }

// hostSharing is the share and the team listener; its mutex guards both.
type hostSharing struct {
	mu   sync.Mutex
	ln   net.Listener
	addr string
	err  string
}

type hostShareConfig struct {
	Teams map[string][]string `json:"teams"` // folders, by team
}

func (s *Server) hostShareFile() string { return filepath.Join(s.dataDir, "share.json") }

func (s *Server) readShare() hostShareConfig {
	var f hostShareConfig
	if data, err := os.ReadFile(s.hostShareFile()); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	if f.Teams == nil {
		f.Teams = map[string][]string{}
	}
	return f
}

// sharedProjects are the folders the host shares with team.
func (s *Server) sharedProjects(team string) []string { return s.readShare().Teams[team] }

// hostShare is what the host shares, and where the listener is.
func (s *Server) hostShare() proto.HostShare {
	out := proto.HostShare{Teams: []proto.HostShareTeam{}}
	f := s.readShare()
	for _, sp := range s.spaces() {
		if list := f.Teams[sp.Team]; len(list) > 0 {
			out.Teams = append(out.Teams, proto.HostShareTeam{Team: sp.Team, Projects: list})
		}
	}
	s.share.mu.Lock()
	out.Addr, out.Err = s.share.addr, s.share.err
	s.share.mu.Unlock()
	return out
}

// setHostShare shares projects with a team, or nothing.
func (s *Server) setHostShare(p proto.HostShareTeam) (proto.HostShare, error) {
	sp, ok := s.spaceByTeam(p.Team)
	if !ok {
		return proto.HostShare{}, fmt.Errorf("spaces: team %s isn't on this host", p.Team)
	}
	if sp.Role == api.RoleViewer {
		return proto.HostShare{}, fmt.Errorf("teams: as a viewer of %s you can't share a host with it", sp.Name)
	}
	known := s.projects.List()
	var paths []string
	for _, path := range p.Projects {
		if !slices.ContainsFunc(known, func(x proto.ProjectInfo) bool { return x.Path == path }) {
			return proto.HostShare{}, fmt.Errorf("teams: %s isn't a project of this host", path)
		}
		if !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	f := s.readShare()
	if len(paths) == 0 {
		delete(f.Teams, p.Team)
	} else {
		f.Teams[p.Team] = paths
	}
	data, _ := json.MarshalIndent(f, "", "  ")
	if err := writeTeamFile(s.hostShareFile(), data); err != nil {
		return proto.HostShare{}, err
	}
	s.startSharing()
	return s.hostShare(), nil
}

// startSharing opens the team listener while something is shared, closes
// it while nothing is, and announces the host in its teams' spaces.
func (s *Server) startSharing() {
	shared := false
	for _, list := range s.readShare().Teams {
		shared = shared || len(list) > 0
	}
	s.share.mu.Lock()
	switch {
	case shared && s.share.ln == nil:
		addr := os.Getenv(proto.EnvTeamListen)
		if addr == "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			ip, err := tailnet.SelfIP(ctx)
			cancel()
			if err != nil || ip == "" {
				s.share.err = "members reach a shared host over Tailscale: " + errText(err, "this host has no Tailscale address")
				break
			}
			addr = net.JoinHostPort(ip, strconv.Itoa(proto.TeamPort))
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			s.share.err = "the team listener: " + err.Error()
			break
		}
		s.share.ln, s.share.addr, s.share.err = ln, ln.Addr().String(), ""
		go listenTeam(s, ln)
	case !shared && s.share.ln != nil:
		s.share.ln.Close()
		s.share.ln, s.share.addr, s.share.err = nil, "", ""
	case !shared:
		s.share.err = ""
	}
	s.share.mu.Unlock()
	s.announceHost()
}

func errText(err error, none string) string {
	if err != nil {
		return err.Error()
	}
	return none
}

// stopSharing closes the team listener, as the daemon stops.
func (s *Server) stopSharing() {
	s.share.mu.Lock()
	defer s.share.mu.Unlock()
	if s.share.ln != nil {
		s.share.ln.Close()
		s.share.ln = nil
	}
}

// announceHost writes, or takes back, the host's entry in each of its
// teams' spaces.
func (s *Server) announceHost() {
	st, err := s.cloud.store.Get()
	if err != nil || !st.SignedIn() || st.DeviceID == "" {
		return
	}
	f := s.readShare()
	s.share.mu.Lock()
	addr := s.share.addr
	s.share.mu.Unlock()
	for _, sp := range s.spaces() {
		file := filepath.Join(s.teamsDir(), sp.Team, "hosts", st.DeviceID+".json")
		list := f.Teams[sp.Team]
		if len(list) == 0 || addr == "" {
			if os.Remove(file) == nil {
				s.teamSync.kick()
			}
			continue
		}
		h := proto.TeamHost{Team: sp.Team, Device: st.DeviceID, Name: s.userdata.Name, Owner: st.Account, Addr: addr, Projects: []proto.TeamHostProject{}}
		for _, path := range list {
			h.Projects = append(h.Projects, proto.TeamHostProject{Name: filepath.Base(path), Path: path, Ref: s.remoteRef(path)})
		}
		var old proto.TeamHost
		if data, err := os.ReadFile(file); err == nil && json.Unmarshal(data, &old) == nil {
			h.Updated = old.Updated
			if same, _ := json.Marshal(h); string(same) == string(data) {
				continue
			}
		}
		h.Updated = time.Now().UTC().Truncate(time.Second)
		data, _ := json.Marshal(h)
		if writeTeamFile(file, data) == nil {
			s.teamSync.kick()
		}
	}
}

// teamHosts are the hosts members share with the host's teams, but its own.
func (s *Server) teamHosts() []proto.TeamHost {
	st, _ := s.cloud.store.Get()
	out := []proto.TeamHost{}
	for _, sp := range s.spaces() {
		entries, _ := os.ReadDir(filepath.Join(s.teamsDir(), sp.Team, "hosts"))
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".json") || strings.TrimSuffix(e.Name(), ".json") == st.DeviceID {
				continue
			}
			var h proto.TeamHost
			data, err := os.ReadFile(filepath.Join(s.teamsDir(), sp.Team, "hosts", e.Name()))
			if err != nil || json.Unmarshal(data, &h) != nil || h.Device != strings.TrimSuffix(e.Name(), ".json") {
				continue
			}
			h.Team = sp.Team
			out = append(out, h)
		}
	}
	return out
}

// visitorScope are the folders a visitor sees: their team's shared ones.
func (c *conn) visitorScope() []string {
	v := c.visitor.Load()
	if v == nil {
		return nil
	}
	return c.srv.sharedProjects(v.Team)
}

// inScope says folder dir is within one of the visitor's shared projects.
func inScope(scope []string, dir string) bool {
	return dir != "" && slices.ContainsFunc(scope, func(p string) bool { return within(dir, p) })
}

var errNotShared = errors.New("that isn't shared with your team on this host")

// visitorOverrides answer a visitor in place of the host's own handlers:
// the same, within their team's shared projects.
var visitorOverrides = map[string]handler{
	proto.MethodProjectList: bare(func(c *conn) (any, error) {
		scope := c.visitorScope()
		out := []proto.ProjectInfo{}
		for _, p := range c.srv.projects.List() {
			if slices.Contains(scope, p.Path) {
				p.Group = ""
				out = append(out, p)
			}
		}
		return out, nil
	}),
	proto.MethodSessionList: bare(func(c *conn) (any, error) {
		scope := c.visitorScope()
		out := []proto.SessionInfo{}
		for _, sess := range c.srv.sessions.List() {
			if info := sess.Info(); visibleTo(info, scope) {
				out = append(out, info)
			}
		}
		return out, nil
	}),
	proto.MethodProjectContext: with(func(c *conn, p proto.ProjectContextParams) (any, error) {
		if p.Host != "" && p.Host != c.srv.daemonID || !inScope(c.visitorScope(), homePath(p.Path)) {
			return nil, errNotShared
		}
		return c.srv.projectContext(proto.ProjectContextParams{Path: p.Path})
	}),
	proto.MethodProjectRead: with(func(c *conn, p proto.ProjectReadParams) (any, error) {
		if p.Host != "" && p.Host != c.srv.daemonID || !inScope(c.visitorScope(), homePath(p.Path)) {
			return nil, errNotShared
		}
		return projectctx.Read(homePath(p.Path), p.File)
	}),
}

var hostShareHandlers = map[string]handler{
	proto.MethodHostShare: bare(func(c *conn) (any, error) {
		if c.remote {
			return nil, errors.New("only the host's own connections change what it shares")
		}
		return c.srv.hostShare(), nil
	}),
	proto.MethodHostShareSet: with(func(c *conn, p proto.HostShareTeam) (any, error) {
		if c.remote {
			return nil, errors.New("only the host's own connections change what it shares")
		}
		return c.srv.setHostShare(p)
	}),
	proto.MethodTeamHosts: bare(func(c *conn) (any, error) {
		if c.remote {
			return nil, errors.New("only the host's own connections list its teams' hosts")
		}
		return c.srv.teamHosts(), nil
	}),
}
