// Package daemon serves the enowX protocol: it owns sessions, projects and
// worktrees and streams terminal frames to every attached client.
package daemon

import (
	"encoding/json"
	"errors"
	"github.com/enowx/enowx/core/accounts"
	"github.com/enowx/enowx/core/checks"
	"github.com/enowx/enowx/core/commands"
	"github.com/enowx/enowx/core/credits"
	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/core/intercept"
	"github.com/enowx/enowx/core/vault"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/version"
)

// Version is the daemon's version, reported in Hello and daemon.info.
var Version = version.String

// Server serves connections. One Server runs per user per machine.
type Server struct {
	exitsLogged sync.Map // session IDs whose end is in the log
	pipes       pipelines
	autos       automations
	// WorktreeRoot is where worktrees are created.
	WorktreeRoot string
	// Hash is the SHA-256 of the executable the daemon started from
	// (DaemonInfo.Hash).
	Hash string
	// HookBin is the program agents' hooks run to report their state
	// ("<HookBin> hook <state>"); empty leaves agents without hooks.
	HookBin string
	// Log receives diagnostics; nil discards them.
	Log *slog.Logger

	sessions  *session.Manager
	projects  *project.Store
	notes     *notes.Store
	clones    clones
	accounts  *accounts.Store // agents' other accounts
	browsers  browsers        // apps doing enowx browser's actions
	vault     vaultState      // apps lending credentials, and what they lent (vault.go)
	persist   *persistence
	started   time.Time
	done      chan struct{}
	closeOnce sync.Once

	transcripts transcripts
	statsState  stats
	credits     credits.Checker
	// commandCache keeps the agents' slash commands (see commands).
	commandCache commandCache
	checkStore   *checks.Store
	vars         sessionVars
	commandStore *commands.Store
	runner       checks.Runner
	sched        scheduler
	displays     displays
	syncHash     projectsync.Hasher // files' hashes, for syncs
	traf         traffic            // the HTTP interceptor (traffic.go)
	android      androidState       // Android devices (android.go)

	mu    sync.Mutex
	conns map[*conn]struct{}
}

// NewServer returns a server keeping its state (projects, worktrees,
// sessions) under dir. Sessions saved by a previous daemon come back.
func NewServer(dir string) (*Server, error) { return newServer(dir, "") }

// newServer is NewServer with the program agents' hooks run (HookBin),
// set before saved sessions come back so they get it too.
func newServer(dir, hookBin string) (*Server, error) {
	projects, err := project.Open(filepath.Join(dir, "projects.json"))
	if err != nil {
		return nil, err
	}
	noteStore := notes.Open(filepath.Join(dir, "notes"))
	persist, err := openPersistence(dir)
	if err != nil {
		return nil, err
	}
	checkStore, err := checks.OpenStore(filepath.Join(dir, "checks.json"))
	if err != nil {
		return nil, err
	}
	commandStore, err := commands.OpenStore(filepath.Join(dir, "commands.json"))
	if err != nil {
		return nil, err
	}
	accountStore, err := accounts.Open(dir)
	if err != nil {
		return nil, err
	}
	s := &Server{
		WorktreeRoot: filepath.Join(dir, "worktrees"),
		HookBin:      hookBin,
		sessions:     session.NewManager(),
		projects:     projects,
		notes:        noteStore,
		persist:      persist,
		checkStore:   checkStore,
		commandStore: commandStore,
		accounts:     accountStore,
		started:      time.Now(),
		conns:        map[*conn]struct{}{},
		done:         make(chan struct{}),
	}
	s.vars.load(s.sessionEnvFile())
	s.vault.audit = vault.OpenAudit(filepath.Join(dir, "vault", "audit.jsonl"))
	intercept.KnownSecrets(s.lentSecrets)
	s.loadShell()
	s.sessions.OnUpdate = func(info proto.SessionInfo) {
		if info.Exited {
			// When and how each program ended, to tell later what ended it
			// (143 is SIGTERM from outside, say).
			if _, logged := s.exitsLogged.LoadOrStore(info.ID, true); !logged {
				s.logger().Info("session ended", "id", info.ID, "agent", info.Agent, "code", info.ExitCode)
			}
		} else {
			s.exitsLogged.Delete(info.ID)
		}
		s.sched.poke()
		if sess, err := s.sessions.Get(info.ID); err == nil {
			s.persist.update(sess, nil)
		}
		s.broadcast(proto.EventSessionUpdated, info)
	}
	if err := s.sched.open(settingsFile(dir)); err != nil {
		return nil, err
	}
	s.runner.OnChange = func(id string) { s.broadcast(proto.EventChecksChanged, proto.SessionRef{ID: id}) }
	// A session's checks run with its traffic caught, when it is, so its
	// project's mocks answer its tests.
	s.runner.Env = func(session string) []string { return s.trafficEnvFor(session, "") }
	s.restore()
	s.pipes.load(pipelinesFile(dir))
	s.resumePipelines()
	s.autos.load(automationsFile(dir))
	s.sweepTransfers()
	s.sweepFetches()
	go s.saveHistories(s.done)
	go s.watchPorts(s.done)
	go s.runScheduler(s.done)
	go s.runAutomations(s.done)
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
		s.browsers.drop(c)
		s.vault.drop(c)
	}()
	if err := c.serve(); err != nil {
		s.logger().Debug("connection closed", "err", err)
	}
}

// Close saves every session for the next daemon, then hangs them up.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		s.clones.stopAll() // half-made clones go with them
		s.persist.stopping()
		var tags []tag
		for _, sess := range s.sessions.List() {
			s.persist.saveHistory(sess)
			tags = append(tags, tag{key: session.EnvSession, value: sess.ID()})
		}
		s.sessions.CloseAll()
		s.runner.StopAll()
		s.stopTraffic()
		s.android.logs.Close()
		s.vaultReleaseAll()
		tags = append(tags, s.displays.stopAll()...)
		// Sessions come back with the next daemon, and start what they
		// run again: nothing of theirs should run twice.
		reapAll(tags)
	})
}

// deleteSession hangs up a session and forgets it for good.
func (s *Server) deleteSession(id string) error {
	if err := s.sessions.Kill(id); err != nil {
		return err
	}
	// What it started outside its terminal goes too: no leftovers.
	reap(tag{key: session.EnvSession, value: id})
	// Its tabs go with it.
	for _, tab := range s.sessions.List() {
		if tab.Meta().Parent == id && s.deleteSession(tab.ID()) == nil {
			s.broadcast(proto.EventSessionRemoved, proto.SessionRef{ID: tab.ID()})
		}
	}
	s.persist.remove(id)
	s.transcripts.forget(id)
	s.runner.Forget(id)
	s.unqueue(id)
	s.displays.stop(id)
	s.vaultRelease(id)
	_ = os.RemoveAll(s.uploadDir(id))
	return nil
}

func (s *Server) info() proto.DaemonInfo {
	host, _ := os.Hostname()
	home, _ := os.UserHomeDir()
	info := proto.DaemonInfo{
		Version:  Version,
		Build:    version.Build(),
		PID:      os.Getpid(),
		Started:  s.started,
		Sessions: len(s.sessions.List()),
		Hostname: host,
		Home:     home,
		Hash:     s.Hash,
	}
	info.Display, info.DisplayNote = display.Available()
	if !info.Display {
		info.DisplayInstall = display.InstallCommand()
	}
	return info
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
