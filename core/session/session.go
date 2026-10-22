// Package session keeps terminal sessions alive in the daemon.
//
// Each session runs a program in a PTY and emulates its screen in a
// headless terminal, so clients can attach, detach and reattach at any time
// and always see the current screen and scrollback, cell for cell.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"github.com/enowx/enowx/proto"
)

// ErrNotFound is returned for an unknown session ID.
var ErrNotFound = errors.New("session: not found")

// Scrollback is how many bytes of scrollback each session keeps.
const Scrollback = 10 << 20

// Session is one running (or exited) program.
type Session struct {
	id      string
	command []string
	dir     string
	created time.Time
	term    *terminal.Terminal

	mu     sync.Mutex
	title  string
	exited bool
	code   int
}

// ID returns the session's ID.
func (s *Session) ID() string { return s.id }

// Terminal returns the session's headless terminal.
func (s *Session) Terminal() *terminal.Terminal { return s.term }

// Info describes the session.
func (s *Session) Info() proto.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return proto.SessionInfo{
		ID:       s.id,
		Command:  slices.Clone(s.command),
		Dir:      s.dir,
		Title:    s.title,
		Created:  s.created,
		Exited:   s.exited,
		ExitCode: s.code,
	}
}

// Manager owns all sessions of a daemon.
type Manager struct {
	// OnUpdate is called (from any goroutine, without locks held) when a
	// session is created, changes title or exits. Set it before Create.
	OnUpdate func(proto.SessionInfo)

	mu       sync.Mutex
	sessions map[string]*Session
	order    []string
}

// NewManager returns an empty manager.
func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
}

// Create starts a session.
func (m *Manager) Create(p proto.SessionCreateParams) (*Session, error) {
	if err := terminal.Load(); err != nil {
		return nil, fmt.Errorf("session: load terminal emulator: %w", err)
	}
	s := &Session{
		id:      newID(),
		command: slices.Clone(p.Command),
		dir:     p.Dir,
		created: time.Now(),
	}
	term, err := terminal.New(terminal.Options{
		Command:    p.Command,
		Dir:        p.Dir,
		Env:        p.Env,
		Scrollback: Scrollback,
		// Programs' clipboard reads go to the attached window to answer.
		ClipboardRead: terminal.ClipboardForward,
		OnTitle: func(title string) {
			s.mu.Lock()
			s.title = title
			s.mu.Unlock()
			m.notify(s)
		},
		OnExit: func(code int) {
			s.mu.Lock()
			s.exited, s.code = true, code
			s.mu.Unlock()
			m.notify(s)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("session: start %v: %w", p.Command, err)
	}
	s.term = term
	// Without a size, the program starts at the first client's size (or at
	// 80x24 after half a second), so it doesn't draw once at a size nobody
	// sees and then redraw.
	if p.Cols > 0 && p.Rows > 0 {
		term.Resize(p.Cols, p.Rows)
	}

	m.mu.Lock()
	m.sessions[s.id] = s
	m.order = append(m.order, s.id)
	m.mu.Unlock()
	m.notify(s)
	return s, nil
}

// Get returns the session with the given ID.
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

// List returns all sessions in creation order.
func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.sessions[id])
	}
	return out
}

// Kill hangs up the session's program and forgets the session.
func (m *Manager) Kill(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
		m.order = slices.DeleteFunc(m.order, func(o string) bool { return o == id })
	}
	m.mu.Unlock()
	if !ok {
		return ErrNotFound
	}
	return s.term.Close()
}

// CloseAll hangs up every session.
func (m *Manager) CloseAll() {
	for _, s := range m.List() {
		_ = m.Kill(s.id)
	}
}

// notify reports a change, except for killed sessions: their program's
// exit after Kill must not announce them again.
func (m *Manager) notify(s *Session) {
	m.mu.Lock()
	_, live := m.sessions[s.id]
	m.mu.Unlock()
	if m.OnUpdate != nil && s.term != nil && live {
		m.OnUpdate(s.Info())
	}
}

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
