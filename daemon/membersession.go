package daemon

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/core/accounts"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// A member's session on another member's host (docs/cloud.md, Hosts and
// sessions): started by a visitor in a project shared with their team, it
// runs with an agent account of their own on the host, never the host's
// user's sign-in; with their git identity; and with none of the host's
// credentials, the vault and its git helper lending nothing to it. It is
// theirs to type into.

func init() {
	visitorMethods[proto.MethodSessionCreate] = api.RoleMember
	visitorOverrides[proto.MethodSessionCreate] = with(func(c *conn, p proto.SessionCreateParams) (any, error) {
		v := c.visitor.Load()
		sess, err := c.srv.memberSession(v, c.visitorScope(), p)
		if err != nil {
			return nil, err
		}
		c.srv.logger().Info("a team's member started a session", "session", sess.ID(), "account", v.Account, "project", p.Project)
		go c.srv.recordAudit(v.Team, api.AuditParams{Action: api.AuditMemberSession, Detail: visitorLabel(*v) + " · " + c.srv.sessionLabel(sess.ID())})
		return sess.Info(), nil
	})
}

// memberSession starts visitor v's session of p, within scope.
func (s *Server) memberSession(v *proto.Visitor, scope []string, p proto.SessionCreateParams) (*session.Session, error) {
	if p.NotesFor != "" || p.CanvasFor != "" || p.Parent != "" || p.SignIn {
		return nil, errors.New("a team's member starts sessions in its shared projects only")
	}
	if !inScope(scope, p.Project) {
		return nil, errNotShared
	}
	base := agent.Base(p.Agent)
	if !accounts.Supported(base) {
		return nil, fmt.Errorf("on another member's host a session runs with your own agent account: Claude Code or Codex, not %s", nonEmptyAgent(p.Agent))
	}
	acc, err := s.memberAccount(base, v)
	if err != nil {
		return nil, err
	}
	p.Account, p.Creds = acc.ID, nil
	p.Env = append(p.Env, memberEnv(v, s.accounts.Home(acc.ID))...)
	member := &proto.SessionMember{Account: v.Account, Name: v.Name, GitHub: v.GitHub, Team: v.Team}
	return s.createSessionFor(p, member)
}

func nonEmptyAgent(a string) string {
	if a == "" {
		return "a shell"
	}
	return a
}

// memberAccount is the host's account of agent for member v, made the
// first time: their own sign-in, kept apart from the host's user's.
func (s *Server) memberAccount(agentID string, v *proto.Visitor) (proto.Account, error) {
	name := "Team · " + v.Name
	if v.GitHub != "" {
		name = "Team · @" + v.GitHub
	}
	for _, a := range s.accounts.List() {
		if a.Agent == agentID && strings.EqualFold(a.Name, name) {
			return a, nil
		}
	}
	return s.accounts.Add(agentID, name)
}

// memberEnv is a member's session's git identity, and their own place for
// the GitHub CLI's sign-in.
func memberEnv(v *proto.Visitor, home string) []string {
	name := v.Name
	if name == "" {
		name = v.GitHub
	}
	email := v.GitHub + "@users.noreply.github.com"
	if v.GitHub == "" {
		email = v.Account + "@users.noreply.enowx.ai"
	}
	return []string{
		"GIT_AUTHOR_NAME=" + name, "GIT_COMMITTER_NAME=" + name,
		"GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_EMAIL=" + email,
		"GH_CONFIG_DIR=" + filepath.Join(home, "gh"),
	}
}

// ownsSession says session id is visitor account's own.
func (s *Server) ownsSession(id, account string) bool {
	sess, err := s.sessions.Get(id)
	if err != nil || account == "" {
		return false
	}
	m := sess.Info().Member
	return m != nil && m.Account == account
}
