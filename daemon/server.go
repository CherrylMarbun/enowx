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
)

// Version is the daemon's version, reported in Hello and daemon.info.
var Version = "0.0.1-dev"

// Server serves connections. One Server runs per user per machine.
type Server struct {
	// WorktreeRoot is where worktrees are created.
	WorktreeRoot string
	// Log receives diagnostics; nil discards them.
	Log *slog.Logger

	sessions *session.Manager
	projects *project.Store
	started  time.Time

	mu    sync.Mutex
	conns map[*conn]struct{}
}

// NewServer returns a server keeping its state (projects, worktrees) under
// dir.
func NewServer(dir string) (*Server, error) {
	projects, err := project.Open(filepath.Join(dir, "projects.json"))
	if err != nil {
		return nil, err
	}
	s := &Server{
		WorktreeRoot: filepath.Join(dir, "worktrees"),
		sessions:     session.NewManager(),
		projects:     projects,
		started:      time.Now(),
		conns:        map[*conn]struct{}{},
	}
	s.sessions.OnUpdate = func(info proto.SessionInfo) {
		s.broadcast(proto.EventSessionUpdated, info)
	}
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

// Close hangs up every session.
func (s *Server) Close() {
	s.sessions.CloseAll()
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
