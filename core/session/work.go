package session

import (
	"time"

	"github.com/enowx/enowx/proto"
)

// maxTick bounds one look's share of the work counted, so a pause of the
// watch (a sleeping laptop) doesn't count as work.
const maxTick = 2 * time.Second

// working reports whether the agent is on a task: as its hooks say, else
// as its screen shows. The caller holds s.mu.
func (s *Session) workingLocked(now time.Time) bool {
	if s.exited {
		return false
	}
	switch s.stateLocked(now) {
	case proto.StateWorking:
		return true
	case proto.StateWaiting, proto.StateDone:
		return false
	}
	return s.active
}

// countWork adds the time since the last look to the work while the agent
// works, and reports whether that reached its budget just now. The caller
// holds s.mu.
func (s *Session) countWork(now time.Time) bool {
	if !s.tick.IsZero() && s.workingLocked(now) {
		s.worked += min(now.Sub(s.tick), maxTick)
	}
	s.tick = now
	if s.meta.Budget > 0 && s.worked >= s.meta.Budget && !s.overBudget {
		s.overBudget = true
		return true
	}
	return false
}

// SetBudget sets how long session s's agent may work, 0 for no limit. A
// budget it is already over interrupts it at the watch's next look.
func (m *Manager) SetBudget(s *Session, budget time.Duration) {
	s.mu.Lock()
	s.meta.Budget = max(budget, 0)
	s.overBudget = false
	s.mu.Unlock()
	m.notify(s)
}

// Replace starts spec in place of the session spec.ID names, keeping its
// place among the sessions: a queued session's agent starting.
func (m *Manager) Replace(spec Spec) (*Session, error) {
	old, err := m.Get(spec.ID)
	if err != nil {
		return nil, err
	}
	if spec.Created.IsZero() {
		spec.Created = old.created
	}
	return m.create(spec, old)
}
