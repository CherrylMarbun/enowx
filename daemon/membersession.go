package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/core/accounts"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// A member's session on another member's host (docs/cloud.md, Hosts and
// sessions): started by a visitor in a project shared with their team,
// once the host's user let the team's members start sessions there (the
// share's AllowSessions: they run as the host's user). It runs with an
// agent account of their own on the host, isolated from the host's
// user's, never their sign-in, settings or conversations; with their git
// identity; and with none of the host's credentials: no variable of the
// host's, no model API key, no vault, nothing the visitor passes. It is
// theirs to type into.

var (
	errSessionsOff = errors.New("this host's user doesn't let the team's members start sessions here: they run as the host's user, so it is theirs to turn on in the team's Hosts tab")
	errNoSetup     = errors.New("a team's member's session runs no setup script of the project's on this host: start it without")
)

func init() {
	visitorMethods[proto.MethodSessionCreate] = api.RoleMember
	visitorOverrides[proto.MethodSessionCreate] = with(func(c *conn, p proto.SessionCreateParams) (any, error) {
		if !c.srv.shareAllowsSessions(c.visitor.Load().Team) {
			return nil, errSessionsOff
		}
		// Who they are now, not when they came in: a session is worth
		// asking the cloud for.
		c.srv.recheckVisitors(c.visitor.Load().Team)
		v := c.visitor.Load()
		if c.isClosed() || rank[v.Role] < rank[api.RoleMember] {
			return nil, fmt.Errorf("starting a session needs a %s of the team", api.RoleMember)
		}
		sess, err := c.srv.memberSession(v, c.visitorScope(), p)
		if err != nil {
			return nil, err
		}
		c.srv.logger().Info("a team's member started a session", "session", sess.ID(), "account", v.Account, "project", p.Project)
		go c.srv.recordAudit(v.Team, api.AuditParams{Action: api.AuditMemberSession, Detail: visitorLabel(*v) + " · " + c.srv.sessionLabel(sess.ID())})
		return forVisitor(sess.Info()), nil
	})
}

// memberSession starts visitor v's session of p, within scope.
func (s *Server) memberSession(v *proto.Visitor, scope []string, p proto.SessionCreateParams) (*session.Session, error) {
	if p.NotesFor != "" || p.CanvasFor != "" || p.Parent != "" || p.SignIn {
		return nil, errors.New("a team's member starts sessions in its shared projects only")
	}
	if p.RunSetup {
		return nil, errNoSetup
	}
	if !slices.Contains(scope, p.Project) {
		return nil, errNotShared // a shared project itself, no folder in it
	}
	base := agent.Base(p.Agent)
	if !accounts.Supported(base) {
		return nil, fmt.Errorf("on another member's host a session runs with your own agent account: Claude Code or Codex, not %s", nonEmptyAgent(p.Agent))
	}
	acc, err := s.memberAccount(base, v)
	if err != nil {
		return nil, err
	}
	member := &proto.SessionMember{Account: v.Account, Name: v.Name, GitHub: v.GitHub, Team: v.Team}
	// What they pass of their own goes: the host's choices alone.
	p.Account, p.Creds, p.Command, p.Shell, p.Dir = acc.ID, nil, nil, "", ""
	p.Env = memberEnv(member, s.accounts.Home(acc.ID))
	return s.createSessionFor(p, member)
}

func nonEmptyAgent(a string) string {
	if a == "" {
		return "a shell"
	}
	return a
}

// memberAccount is the host's account of agent for member v, made the
// first time: their own sign-in, kept apart from the host's user's, found
// by their cloud account (a login can change hands), labelled with it.
func (s *Server) memberAccount(agentID string, v *proto.Visitor) (proto.Account, error) {
	if a, ok := s.accounts.ForMember(agentID, v.Account); ok {
		return a, nil
	}
	name := "Team · " + v.Name
	if v.GitHub != "" {
		name = "Team · @" + v.GitHub
	}
	return s.accounts.AddMember(agentID, name, v.Account)
}

// memberEnv is a member's session's git identity, and their own place for
// the GitHub CLI's sign-in, in home (their account's).
func memberEnv(m *proto.SessionMember, home string) []string {
	name := m.Name
	if name == "" {
		name = m.GitHub
	}
	email := m.GitHub + "@users.noreply.github.com"
	if m.GitHub == "" {
		email = m.Account + "@users.noreply.enowx.ai"
	}
	return []string{
		"GIT_AUTHOR_NAME=" + name, "GIT_COMMITTER_NAME=" + name,
		"GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_EMAIL=" + email,
		"GH_CONFIG_DIR=" + filepath.Join(home, "gh"),
	}
}

// memberKeeps are the variables of the daemon's environment a member's
// session keeps: what programs need to run, nothing of its user's.
var memberKeeps = []string{
	"PATH", "HOME", "USER", "LOGNAME", "SHELL", "LANG", "LANGUAGE", "TZ", "TERM", "COLORTERM", "TMPDIR", "TMP", "TEMP",
	// Windows'
	"SYSTEMROOT", "SystemRoot", "WINDIR", "windir", "COMSPEC", "ComSpec", "PATHEXT", "SystemDrive", "SYSTEMDRIVE",
	"USERPROFILE", "APPDATA", "LOCALAPPDATA", "PROGRAMDATA", "ProgramData", "ProgramFiles", "ProgramFiles(x86)", "ProgramW6432",
	"CommonProgramFiles", "CommonProgramFiles(x86)", "CommonProgramW6432", "HOMEDRIVE", "HOMEPATH", "USERNAME", "OS",
	"NUMBER_OF_PROCESSORS", "PROCESSOR_ARCHITECTURE",
}

// memberEnviron is env, a member's session's, with each variable of the
// daemon's own environment emptied first, but memberKeeps, locales and
// those env sets: a session's program starts from the daemon's
// environment, its user's, secrets maybe (an SSH agent, tokens).
func memberEnviron(env []string) []string {
	set := map[string]bool{}
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		set[k] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if k == "" || set[k] || slices.Contains(memberKeeps, k) || strings.HasPrefix(k, "LC_") {
			continue
		}
		out = append(out, k+"=")
	}
	return append(out, env...)
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
