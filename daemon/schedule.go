package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// The scheduler keeps at most DaemonSettings.MaxAgents agents working at
// once: an agent session started past it waits, queued, until one
// finishes its turn. It also interrupts agents past their time budget.
// One goroutine (run) does both, woken by session updates, so starting a
// session never happens inside another's update.

// warmup is how long a started agent counts as working before it says:
// an agent with a first prompt starts on it, and the next one queued
// shouldn't start in the meantime.
const warmup = 10 * time.Second

type scheduler struct {
	wake chan struct{}

	mu          sync.Mutex
	path        string
	settings    proto.DaemonSettings
	queue       []queued
	started     map[string]time.Time // agent sessions started, by ID
	interrupted map[string]bool      // sessions interrupted for their budget
}

// queued is a session waiting for its agent to start.
type queued struct {
	spec    session.Spec
	command []string // as persisted
	typed   string   // prompt typed in once the agent is ready
}

func (sc *scheduler) open(path string) error {
	sc.path, sc.wake = path, make(chan struct{}, 1)
	sc.started, sc.interrupted = map[string]time.Time{}, map[string]bool{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("daemon: settings: %w", err)
	}
	return json.Unmarshal(data, &sc.settings)
}

func (sc *scheduler) poke() {
	select {
	case sc.wake <- struct{}{}:
	default:
	}
}

func (sc *scheduler) get() proto.DaemonSettings {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.settings
}

func (sc *scheduler) set(st proto.DaemonSettings) (proto.DaemonSettings, error) {
	st.MaxAgents = max(st.MaxAgents, 0)
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return proto.DaemonSettings{}, err
	}
	if err := os.WriteFile(sc.path, data, 0o600); err != nil {
		return proto.DaemonSettings{}, fmt.Errorf("daemon: settings: %w", err)
	}
	sc.mu.Lock()
	sc.settings = st
	sc.mu.Unlock()
	sc.poke()
	return st, nil
}

// working reports whether a session's agent is on a task, as clients see.
func working(info proto.SessionInfo) bool {
	if info.Exited || info.Queued {
		return false
	}
	switch info.State {
	case proto.StateWorking:
		return true
	case "":
		return info.Active
	}
	return false
}

// busyAgents counts the agents working, or just started.
func (s *Server) busyAgents() int {
	sc := &s.sched
	now := time.Now()
	n := 0
	for _, sess := range s.sessions.List() {
		info := sess.Info()
		if _, ok := agent.Find(info.Agent); !ok || info.Exited || info.Queued {
			continue
		}
		sc.mu.Lock()
		at := sc.started[info.ID]
		sc.mu.Unlock()
		if working(info) || now.Sub(at) < warmup {
			n++
		}
	}
	return n
}

// mustQueue reports whether an agent session starting now waits: the host
// has a limit it reached, or others wait already.
func (s *Server) mustQueue() bool {
	sc := &s.sched
	sc.mu.Lock()
	limit, waiting := sc.settings.MaxAgents, len(sc.queue)
	sc.mu.Unlock()
	return limit > 0 && (waiting > 0 || s.busyAgents() >= limit)
}

// enqueue shows spec as a queued session until its turn.
func (s *Server) enqueue(spec session.Spec, command []string, typed string) (*session.Session, error) {
	spec.ID = session.NewID()
	held := spec
	held.Command, held.Env = nil, nil
	held.Meta.Queued = true
	limit := s.sched.get().MaxAgents
	held.History = fmt.Appendf(nil, "\x1b[2mQueued: %d %s already working on this host. This one starts when one finishes its turn.\x1b[0m\r\n",
		limit, map[bool]string{true: "agent is", false: "agents are"}[limit == 1])
	sess, err := s.sessions.Create(held)
	if err != nil {
		return nil, err
	}
	s.sched.mu.Lock()
	s.sched.queue = append(s.sched.queue, queued{spec: spec, command: command, typed: typed})
	s.sched.mu.Unlock()
	return sess, nil
}

// unqueue forgets session id's place in the queue, if it has one.
func (s *Server) unqueue(id string) {
	s.sched.mu.Lock()
	s.sched.queue = slices.DeleteFunc(s.sched.queue, func(q queued) bool { return q.spec.ID == id })
	delete(s.sched.started, id)
	delete(s.sched.interrupted, id)
	s.sched.mu.Unlock()
}

// runScheduler starts queued agents as others finish, and interrupts those
// past their budget, until done closes.
func (s *Server) runScheduler(done <-chan struct{}) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-s.sched.wake:
		case <-t.C:
		}
		s.startQueued()
		s.enforceBudgets()
	}
}

// startQueued starts queued agents while fewer than the limit work.
func (s *Server) startQueued() {
	sc := &s.sched
	for {
		sc.mu.Lock()
		limit, waiting := sc.settings.MaxAgents, len(sc.queue)
		sc.mu.Unlock()
		if waiting == 0 || limit > 0 && s.busyAgents() >= limit {
			return
		}
		sc.mu.Lock()
		q := sc.queue[0]
		sc.queue = sc.queue[1:]
		sc.mu.Unlock()
		if _, err := s.sessions.Get(q.spec.ID); err != nil {
			continue // deleted while it waited
		}
		sess, err := s.sessions.Replace(q.spec)
		if err != nil {
			s.logger().Warn("could not start a queued session", "id", q.spec.ID, "err", err)
			continue
		}
		s.began(sess, q.command, q.typed)
	}
}

// began records a session whose program started: for a restart, and for
// the scheduler, and types its first prompt once it is ready.
func (s *Server) began(sess *session.Session, command []string, typed string) {
	s.persist.update(sess, command)
	if _, ok := agent.Find(sess.Info().Agent); ok {
		s.sched.mu.Lock()
		s.sched.started[sess.ID()] = time.Now()
		s.sched.mu.Unlock()
	}
	if typed != "" {
		go func() {
			if sess.WaitQuiet(promptQuiet, promptTimeout) {
				s.sessions.Input(sess, []byte(typed+"\r"))
			}
		}()
	}
}

// enforceBudgets interrupts each agent that just went past its budget.
func (s *Server) enforceBudgets() {
	sc := &s.sched
	for _, sess := range s.sessions.List() {
		info := sess.Info()
		sc.mu.Lock()
		done := sc.interrupted[info.ID]
		if !info.OverBudget {
			delete(sc.interrupted, info.ID)
		}
		sc.mu.Unlock()
		if !info.OverBudget || done || info.Exited {
			continue
		}
		sc.mu.Lock()
		sc.interrupted[info.ID] = true
		sc.mu.Unlock()
		key := "\x03"
		if preset, ok := agent.Find(info.Agent); ok {
			key = preset.InterruptKey()
		}
		s.sessions.Input(sess, []byte(key))
	}
}

// settingsFile is where the daemon keeps the host's settings.
func settingsFile(dir string) string { return filepath.Join(dir, "settings.json") }
