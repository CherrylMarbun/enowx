package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/core/hooks"
	"github.com/enowx/enowx/proto"
)

// Where hooks are kept besides the host's own: a project's file, allowed
// as written; a session's own, until it ends; and the changes agents
// proposed, waiting for the user.

// hooksInfo is the host's hooks, the allowed hooks of its projects (and
// of the project in dir), the sessions' own, the project files and
// proposals waiting for the user, with how each agent gets events.
func (s *Server) hooksInfo(dir string) proto.HooksInfo {
	info := proto.HooksInfo{Hooks: s.hooks.store.List(), Support: map[string]map[string]string{}, Host: s.userdata.Name}
	dirs := []string{}
	for _, p := range s.projects.List() {
		dirs = append(dirs, p.Path)
	}
	if dir != "" && !slices.Contains(dirs, dir) {
		dirs = append(dirs, dir)
	}
	s.hooks.mu.Lock()
	for _, d := range dirs {
		ph, allowed := s.hooks.store.Project(d)
		switch {
		case allowed && len(ph.Hooks) > 0:
			info.Projects = append(info.Projects, ph)
		case !allowed && len(ph.Hooks) > 0:
			s.hooks.pending[d] = ph
		}
	}
	for _, ph := range s.hooks.pending {
		info.Pending = append(info.Pending, ph)
	}
	for id, list := range s.hooks.sessions {
		if len(list) > 0 {
			info.Sessions = append(info.Sessions, proto.SessionHooks{Session: id, Hooks: slices.Clone(list)})
		}
	}
	info.Proposals = slices.Clone(s.hooks.proposals)
	s.hooks.mu.Unlock()
	slices.SortFunc(info.Pending, func(a, b proto.ProjectHooks) int { return strings.Compare(a.Dir, b.Dir) })
	slices.SortFunc(info.Sessions, func(a, b proto.SessionHooks) int { return strings.Compare(a.Session, b.Session) })
	for _, p := range agent.Presets {
		if id := agent.Base(p.ID); info.Support[id] == nil {
			info.Support[id] = agent.HookSupport(id)
		}
	}
	return info
}

// approveHooks allows, or stops allowing, a project's hook file.
func (s *Server) approveHooks(p proto.HooksApproval) error {
	if err := s.hooks.store.Allow(p.Dir, p.Hash, p.Allow); err != nil {
		return err
	}
	s.hooks.mu.Lock()
	delete(s.hooks.pending, p.Dir)
	s.hooks.mu.Unlock()
	s.broadcast(proto.EventHooksChanged, nil)
	return nil
}

// saveHostHooks replaces the host's hooks.
func (s *Server) saveHostHooks(list []proto.Hook) ([]proto.Hook, error) {
	out, err := s.hooks.store.Save(list)
	if err == nil {
		s.broadcast(proto.EventHooksChanged, nil)
	}
	return out, err
}

// saveProjectHooks writes the hooks of the project in p.Dir to its
// .enowx/hooks.json, allowed as the user wrote them.
func (s *Server) saveProjectHooks(p proto.ProjectHooks) error {
	if fi, err := os.Stat(p.Dir); err != nil || !fi.IsDir() {
		return fmt.Errorf("hooks: no project folder %s", p.Dir)
	}
	list, err := hooks.Prepare(p.Hooks)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Project = ""
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(p.Dir, hooks.ProjectFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	if err := envsync.WriteFileAtomic(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	ph, _ := s.hooks.store.Project(p.Dir)
	return s.approveHooks(proto.HooksApproval{Dir: p.Dir, Hash: ph.Hash, Allow: true})
}

// saveSessionHooks replaces session p.Session's own hooks.
func (s *Server) saveSessionHooks(p proto.SessionHooks) error {
	if _, err := s.sessions.Get(p.Session); err != nil {
		return err
	}
	list, err := hooks.Prepare(p.Hooks)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Session, list[i].Project = p.Session, ""
	}
	s.hooks.mu.Lock()
	s.hooks.sessions[p.Session] = list
	s.hooks.mu.Unlock()
	s.broadcast(proto.EventHooksChanged, nil)
	return nil
}

// propose keeps an agent's proposed change for the user, and asks the
// apps to say so.
func (s *Server) propose(p proto.HookProposal) (proto.HookProposal, error) {
	if !p.Remove {
		if err := hooks.Check(p.Hook); err != nil {
			return p, err
		}
	}
	switch p.Scope {
	case "", proto.HookScopeHost:
		p.Scope = proto.HookScopeHost
	case proto.HookScopeProject:
		if p.Dir == "" {
			return p, errors.New("hooks: a project's hook needs the project's folder")
		}
	case proto.HookScopeSession:
		if p.Session == "" {
			p.Session = p.By
		}
	default:
		return p, fmt.Errorf("hooks: no scope %q: host, project or session", p.Scope)
	}
	var b [5]byte
	_, _ = rand.Read(b[:])
	p.ID, p.Time = hex.EncodeToString(b[:]), time.Now()
	s.hooks.mu.Lock()
	s.hooks.proposals = append(s.hooks.proposals, p)
	s.hooks.mu.Unlock()
	s.saveProposals()
	s.broadcast(proto.EventHooksChanged, nil)
	what := "add a hook"
	switch {
	case p.Remove:
		what = "remove a hook"
	case p.Hook.ID != "":
		what = "change a hook"
	}
	s.broadcast(proto.EventNotifySound, proto.NotifySoundParams{Kind: "input", Session: p.By, Text: "An agent asks to " + what + ": Settings → Hooks"})
	return p, nil
}

// decide applies proposal p.ID when the user allows it, and forgets it.
func (s *Server) decide(p proto.HookDecideParams) error {
	s.hooks.mu.Lock()
	i := slices.IndexFunc(s.hooks.proposals, func(x proto.HookProposal) bool { return x.ID == p.ID })
	if i < 0 {
		s.hooks.mu.Unlock()
		return fmt.Errorf("hooks: no proposal %s", p.ID)
	}
	pr := s.hooks.proposals[i]
	s.hooks.proposals = slices.Delete(s.hooks.proposals, i, i+1)
	s.hooks.mu.Unlock()
	s.saveProposals()
	defer s.broadcast(proto.EventHooksChanged, nil)
	if !p.Allow {
		return nil
	}
	switch pr.Scope {
	case proto.HookScopeProject:
		ph, _ := s.hooks.store.Project(pr.Dir)
		return s.saveProjectHooks(proto.ProjectHooks{Dir: pr.Dir, Hooks: applyProposal(ph.Hooks, pr)})
	case proto.HookScopeSession:
		s.hooks.mu.Lock()
		list := slices.Clone(s.hooks.sessions[pr.Session])
		s.hooks.mu.Unlock()
		return s.saveSessionHooks(proto.SessionHooks{Session: pr.Session, Hooks: applyProposal(list, pr)})
	}
	_, err := s.saveHostHooks(applyProposal(s.hooks.store.List(), pr))
	return err
}

// applyProposal is list with proposal p made: its hook added, put in
// place of the one with its ID, or that one removed.
func applyProposal(list []proto.Hook, p proto.HookProposal) []proto.Hook {
	i := -1
	if p.Hook.ID != "" {
		i = slices.IndexFunc(list, func(h proto.Hook) bool { return h.ID == p.Hook.ID })
	}
	switch {
	case p.Remove && i >= 0:
		return slices.Delete(list, i, i+1)
	case p.Remove:
		return list
	case i >= 0:
		list[i] = p.Hook
		return list
	}
	return append(list, p.Hook)
}

func (e *hookEngine) loadProposals() {
	if data, err := os.ReadFile(e.propose); err == nil {
		_ = json.Unmarshal(data, &e.proposals)
	}
}

func (s *Server) saveProposals() {
	s.hooks.mu.Lock()
	data, err := json.MarshalIndent(s.hooks.proposals, "", "  ")
	s.hooks.mu.Unlock()
	if err == nil {
		_ = envsync.WriteFileAtomic(s.hooks.propose, data, 0o600)
	}
}

// forgetSession drops what the hooks kept of session id, ended.
func (s *Server) forgetSession(id string) {
	s.hooks.mu.Lock()
	_, had := s.hooks.sessions[id]
	delete(s.hooks.sessions, id)
	s.hooks.mu.Unlock()
	s.hooks.limits.Forget(id)
	if had {
		s.broadcast(proto.EventHooksChanged, nil)
	}
}
