package daemon

import (
	"slices"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/hooks"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// The user's hooks run here: on events an agent's own hooks hand over
// (session.hook) and on those the daemon sees from outside (hooks_watch.go).
// See docs/hooks.md.

// hookLogMax is how many runs the log keeps.
const hookLogMax = 200

// hookEngine runs the host's and its projects' hooks and keeps their log
// and the project hook files waiting for the user. Safe from any
// goroutine.
type hookEngine struct {
	store *hooks.Store

	mu      sync.Mutex
	log     []proto.HookRun // the latest last
	pending map[string]proto.ProjectHooks
	watch   map[string]*sessionWatch // by session ID (hooks_watch.go)
}

func newHookEngine(dir string) *hookEngine {
	return &hookEngine{store: hooks.Open(dir), pending: map[string]proto.ProjectHooks{}, watch: map[string]*sessionWatch{}}
}

// blocking are the events whose action an agent's hooks can still
// refuse, or add context to.
func blocking(event string) bool {
	return event == proto.HookToolBefore || event == proto.HookPromptStart || event == proto.HookSessionStart
}

// eventOf is event e of session info, filled in with the session's agent
// and folders, what the agent sent masked of the vault's secrets.
func (s *Server) eventOf(info proto.SessionInfo, e proto.HookEvent) proto.HookEvent {
	e.Session, e.Agent, e.Dir, e.Branch = info.ID, agent.Base(info.Agent), info.Dir, info.Branch
	e.Project = info.Project
	if e.Project == "" {
		e.Project = info.Dir
	}
	e.Prompt, e.Command = s.maskLent(e.Prompt), s.maskLent(e.Command)
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	return e
}

// hooksFor are the hooks that may run in folder dir: the host's, then the
// project's own once the user allowed its file; a file not allowed yet
// waits in the pending list.
func (s *Server) hooksFor(dir string) []proto.Hook {
	list := s.hooks.store.List()
	if dir == "" {
		return list
	}
	ph, allowed := s.hooks.store.Project(dir)
	s.hooks.mu.Lock()
	if len(ph.Hooks) > 0 && !allowed {
		s.hooks.pending[dir] = ph
	} else {
		delete(s.hooks.pending, dir)
	}
	s.hooks.mu.Unlock()
	if allowed {
		list = append(list, ph.Hooks...)
	}
	return list
}

// fireHooks runs the hooks matching event ev of session info and says
// what the agent is to do: the first refusal stops the rest; contexts add
// up. Hooks of events nothing waits on, and those set to, run in the
// background.
func (s *Server) fireHooks(info proto.SessionInfo, ev proto.HookEvent) proto.HookDecision {
	ev = s.eventOf(info, ev)
	matched := hooks.For(s.hooksFor(info.Dir), ev)
	var d proto.HookDecision
	if len(matched) == 0 {
		return d
	}
	env := []string{session.EnvSession + "=" + info.ID}
	if s.HookBin != "" {
		env = append(env, agent.EnvBin+"="+s.HookBin)
	}
	wait := ev.Via == proto.HookInAgent && blocking(ev.Event)
	var contexts []string
	for _, h := range matched {
		if !wait || h.Background {
			go func() {
				run, _ := hooks.Run(h, ev, env)
				run.Blocked = false // nothing waits on it to refuse
				s.logHookRun(run)
			}()
			continue
		}
		run, hd := hooks.Run(h, ev, env)
		s.logHookRun(run)
		if hd.Block {
			return proto.HookDecision{Block: true, Reason: hd.Reason}
		}
		if hd.Context != "" {
			contexts = append(contexts, hd.Context)
		}
	}
	d.Context = joinContexts(contexts)
	return d
}

func joinContexts(list []string) string {
	out := ""
	for i, c := range list {
		if i > 0 {
			out += "\n\n"
		}
		out += c
	}
	return out
}

// logHookRun keeps run in the log, the oldest dropped past hookLogMax.
func (s *Server) logHookRun(run proto.HookRun) {
	s.hooks.mu.Lock()
	s.hooks.log = append(s.hooks.log, run)
	if len(s.hooks.log) > hookLogMax {
		s.hooks.log = slices.Clone(s.hooks.log[len(s.hooks.log)-hookLogMax:])
	}
	s.hooks.mu.Unlock()
}

// hooksInfo is the host's hooks, those of the project in dir and the
// project files waiting for the user, with how each agent gets events.
func (s *Server) hooksInfo(dir string) proto.HooksInfo {
	info := proto.HooksInfo{Hooks: s.hooks.store.List(), Support: map[string]map[string]string{}}
	if dir != "" {
		if ph, allowed := s.hooks.store.Project(dir); allowed {
			info.Hooks = append(info.Hooks, ph.Hooks...)
		} else if len(ph.Hooks) > 0 {
			s.hooks.mu.Lock()
			s.hooks.pending[dir] = ph
			s.hooks.mu.Unlock()
		}
	}
	s.hooks.mu.Lock()
	for _, ph := range s.hooks.pending {
		info.Pending = append(info.Pending, ph)
	}
	s.hooks.mu.Unlock()
	slices.SortFunc(info.Pending, func(a, b proto.ProjectHooks) int {
		if a.Dir < b.Dir {
			return -1
		}
		return 1
	})
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
	return nil
}

// hookLog is the runs kept, the latest first.
func (s *Server) hookLog() []proto.HookRun {
	s.hooks.mu.Lock()
	defer s.hooks.mu.Unlock()
	out := slices.Clone(s.hooks.log)
	slices.Reverse(out)
	return out
}

// testHook runs hook t.Hook on its sample event now, as an agent would
// wait on it, and logs it.
func (s *Server) testHook(t proto.HookTest) (proto.HookRun, error) {
	if err := hooks.Check(t.Hook); err != nil {
		return proto.HookRun{}, err
	}
	ev := t.Event
	if ev.Event == "" {
		ev.Event = t.Hook.Event
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	env := []string{}
	if s.HookBin != "" {
		env = append(env, agent.EnvBin+"="+s.HookBin)
	}
	run, _ := hooks.Run(t.Hook, ev, env)
	s.logHookRun(run)
	return run, nil
}
