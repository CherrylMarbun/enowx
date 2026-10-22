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
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"github.com/enowx/enowx/proto"
)

// ErrNotFound is returned for an unknown session ID.
var ErrNotFound = errors.New("session: not found")

// Scrollback is how many bytes of scrollback each session keeps.
const Scrollback = 10 << 20

// A session is active while its screen changed within activeFor; the
// screen is checked every activityPeriod. Changes within echoFor of the
// user typing are the echo of their keys, not the program working.
const (
	activeFor      = 1500 * time.Millisecond
	activityPeriod = 150 * time.Millisecond
	echoFor        = 400 * time.Millisecond
)

// Meta describes what a session is for. The daemon fills it from the
// request; the session only reports it.
type Meta struct {
	Name   string // given by the user; empty lets clients pick a label
	Prompt string // first prompt, if given at creation
	// Relaunch says the program comes back after the daemon restarts, as
	// Restart (an agent continuing its conversation; nil: the login shell).
	Relaunch        bool
	Restart         []string
	SkipPermissions bool
	Restored        bool
	Project         string
	Agent           string
	Branch          string
	Worktree        bool
}

// Session is one running (or exited) program.
type Session struct {
	id      string
	meta    Meta
	command []string
	dir     string
	created time.Time
	term    *terminal.Terminal
	done    chan struct{}

	mu         sync.Mutex
	title      string
	exited     bool
	code       int
	lastChange time.Time
	lastInput  time.Time
	active     bool
	prompt     string
	typed      []rune // the line being typed, until the first prompt is known
	inEscape   bool
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
		ID:              s.id,
		Name:            s.meta.Name,
		Project:         s.meta.Project,
		Agent:           s.meta.Agent,
		Branch:          s.meta.Branch,
		Worktree:        s.meta.Worktree,
		Command:         slices.Clone(s.command),
		Dir:             s.dir,
		Title:           s.title,
		Prompt:          s.prompt,
		Created:         s.created,
		Active:          s.active,
		SkipPermissions: s.meta.SkipPermissions,
		Restored:        s.meta.Restored,
		Exited:          s.exited,
		ExitCode:        s.code,
	}
}

// Meta returns what the session is for.
func (s *Session) Meta() Meta {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.meta
	m.Prompt = s.prompt
	return m
}

// History returns the scrollback and screen, to show above a relaunched
// program's output after a restart.
func (s *Session) History() []byte { return s.term.History() }

// NoteInput records what the user typed: its echo must not count as
// activity, and its first line becomes the session's prompt when none was
// given. It reports whether the prompt was just set.
func (s *Session) NoteInput(p []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastInput = time.Now()
	if s.prompt != "" {
		return false
	}
	for _, r := range string(p) {
		switch {
		case s.inEscape:
			// CSI and SS3 sequences end with a letter or ~; others after one rune.
			if r >= '@' && r <= '~' && r != '[' && r != 'O' {
				s.inEscape = false
			}
		case r == 0x1b:
			s.inEscape = true
		case r == '\r' || r == '\n':
			if line := strings.TrimSpace(string(s.typed)); line != "" {
				s.prompt, s.typed = truncate(line, 200), nil
				return true
			}
			s.typed = nil
		case r == 0x7f || r == 0x08:
			if len(s.typed) > 0 {
				s.typed = s.typed[:len(s.typed)-1]
			}
		case r == 0x15: // Ctrl-U
			s.typed = nil
		case r >= 0x20:
			if len(s.typed) < 400 {
				s.typed = append(s.typed, r)
			}
		}
	}
	return false
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// WaitQuiet waits until the screen has changed at least once and then
// stayed still for quiet, or until timeout. It reports whether the session
// settled; programs show their prompt and then wait, which is when typing
// into them is safe.
func (s *Session) WaitQuiet(quiet, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		last, exited := s.lastChange, s.exited
		s.mu.Unlock()
		if exited {
			return false
		}
		if !last.IsZero() && time.Since(last) >= quiet {
			return true
		}
		select {
		case <-s.done:
			return false
		case <-time.After(100 * time.Millisecond):
		}
	}
	return false
}

// Manager owns all sessions of a daemon.
type Manager struct {
	// OnUpdate is called (from any goroutine, without locks held) when a
	// session is created, changes title, becomes active or idle, or exits.
	// Set it before Create.
	OnUpdate func(proto.SessionInfo)

	mu       sync.Mutex
	sessions map[string]*Session
	order    []string
}

// NewManager returns an empty manager.
func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
}

// Spec describes a session to start.
type Spec struct {
	ID         string   // empty for a new session; set when restoring one
	Command    []string // the login shell when empty
	Dir        string
	Env        []string
	Cols, Rows int
	Meta       Meta
	Created    time.Time // zero for now
	// History is shown above the program's output: the screen of the
	// session before the daemon restarted.
	History []byte
	// Exited restores a session whose program had ended: it shows History
	// and runs nothing.
	Exited   bool
	ExitCode int
}

// Create starts a session.
func (m *Manager) Create(spec Spec) (*Session, error) {
	if err := terminal.Load(); err != nil {
		return nil, fmt.Errorf("session: load terminal emulator: %w", err)
	}
	s := &Session{
		id:      spec.ID,
		meta:    spec.Meta,
		command: slices.Clone(spec.Command),
		dir:     spec.Dir,
		created: spec.Created,
		done:    make(chan struct{}),
		prompt:  truncate(strings.TrimSpace(spec.Meta.Prompt), 200),
		exited:  spec.Exited,
		code:    spec.ExitCode,
	}
	if s.id == "" {
		s.id = newID()
	}
	if s.created.IsZero() {
		s.created = time.Now()
	}
	opts := terminal.Options{
		Command:    spec.Command,
		Dir:        spec.Dir,
		Env:        spec.Env,
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
			s.exited, s.code, s.active = true, code, false
			s.mu.Unlock()
			m.notify(s)
		},
	}
	if spec.Exited {
		opts.Conn = newIdleConn() // nothing runs; the history is all it shows
	}
	term, err := terminal.New(opts)
	if err != nil {
		return nil, fmt.Errorf("session: start %v: %w", spec.Command, err)
	}
	s.term = term
	if len(spec.History) > 0 {
		term.Feed(spec.History)
		if !spec.Exited {
			term.Feed([]byte("\r\n\x1b[2m── enowX restarted this session ──\x1b[0m\r\n"))
		}
	}
	// Without a size, the program starts at the first client's size (or at
	// 80x24 after half a second), so it doesn't draw once at a size nobody
	// sees and then redraw.
	if spec.Cols > 0 && spec.Rows > 0 {
		term.Resize(spec.Cols, spec.Rows)
	}

	m.mu.Lock()
	m.sessions[s.id] = s
	m.order = append(m.order, s.id)
	m.mu.Unlock()
	if !spec.Exited {
		go m.watchActivity(s)
	}
	m.notify(s)
	return s, nil
}

// watchActivity follows the screen through a mirror of its own and reports
// when the session starts or stops producing output.
func (m *Manager) watchActivity(s *Session) {
	mirror, err := s.term.NewMirror()
	if err != nil {
		return
	}
	defer mirror.Close()
	t := time.NewTicker(activityPeriod)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-s.term.Done():
			return
		case <-t.C:
		}
		frame, due := mirror.Frame()
		now := time.Now()
		s.mu.Lock()
		if (len(frame) > 0 || due) && now.Sub(s.lastInput) > echoFor {
			s.lastChange = now
		}
		active := !s.exited && !s.lastChange.IsZero() && now.Sub(s.lastChange) < activeFor
		changed := active != s.active
		s.active = active
		s.mu.Unlock()
		if changed {
			m.notify(s)
		}
	}
}

// Input types p into the session and records it (see NoteInput).
func (m *Manager) Input(s *Session, p []byte) {
	s.term.Send(p)
	if s.NoteInput(p) {
		m.notify(s)
	}
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
	close(s.done)
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
