// Package daemon serves the enowX protocol: it owns sessions, projects and
// worktrees and streams terminal frames to every attached client.
package daemon

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/version"
)

// Version is the daemon's version, reported in Hello and daemon.info.
var Version = version.String

// Server serves connections. One Server runs per user per machine.
type Server struct {
	// WorktreeRoot is where worktrees are created.
	WorktreeRoot string
	// Log receives diagnostics; nil discards them.
	Log *slog.Logger

	sessions  *session.Manager
	projects  *project.Store
	persist   *persistence
	started   time.Time
	done      chan struct{}
	closeOnce sync.Once

	transcripts transcripts
	statsState  stats

	mu    sync.Mutex
	conns map[*conn]struct{}
}

// NewServer returns a server keeping its state (projects, worktrees,
// sessions) under dir. Sessions saved by a previous daemon come back.
func NewServer(dir string) (*Server, error) {
	projects, err := project.Open(filepath.Join(dir, "projects.json"))
	if err != nil {
		return nil, err
	}
	persist, err := openPersistence(dir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		WorktreeRoot: filepath.Join(dir, "worktrees"),
		sessions:     session.NewManager(),
		projects:     projects,
		persist:      persist,
		started:      time.Now(),
		conns:        map[*conn]struct{}{},
		done:         make(chan struct{}),
	}
	s.sessions.OnUpdate = func(info proto.SessionInfo) {
		if sess, err := s.sessions.Get(info.ID); err == nil {
			s.persist.update(sess, nil)
		}
		s.broadcast(proto.EventSessionUpdated, info)
	}
	s.restore()
	go s.saveHistories(s.done)
	return s, nil
}

// Serve accepts connections until l is closed.
func (s *Server) Serve(l net.Listener) error {
	for {
		nc, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.ServeConn(nc)
	}
}

// ServeConn serves one connection until it closes.
func (s *Server) ServeConn(nc net.Conn) {
	c := newConn(s, nc)
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
	}()
	if err := c.serve(); err != nil {
		s.logger().Debug("connection closed", "err", err)
	}
}

// Close saves every session for the next daemon, then hangs them up.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		s.persist.stopping()
		for _, sess := range s.sessions.List() {
			s.persist.saveHistory(sess)
		}
		s.sessions.CloseAll()
	})
}

// deleteSession hangs up a session and forgets it for good.
func (s *Server) deleteSession(id string) error {
	if err := s.sessions.Kill(id); err != nil {
		return err
	}
	s.persist.remove(id)
	s.transcripts.forget(id)
	return nil
}

func (s *Server) info() proto.DaemonInfo {
	host, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	return proto.DaemonInfo{
		Version:  Version,
		PID:      os.Getpid(),
		Started:  s.started,
		Sessions: len(s.sessions.List()),
		Hostname: host,
		Home:     home,
	}
}

func (s *Server) broadcast(name string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	ev, err := json.Marshal(proto.Event{Name: name, Data: raw})
	if err != nil {
		return
	}
	s.mu.Lock()
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		c.send(proto.Frame{Type: proto.TypeEvent, Payload: ev})
	}
}

func (s *Server) logger() *slog.Logger {
	if s.Log != nil {
		return s.Log
	}
	return slog.New(slog.DiscardHandler)
}
