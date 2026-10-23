package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/session"
)

// Sessions outlive the daemon: their records live in sessions.json and the
// screens they showed in history/<id>.vt. A restarted daemon brings each
// one back, relaunching agents on their last conversation.
const (
	historySaveEvery = 20 * time.Second
	historyMaxBytes  = 2 << 20
)

// record is a session as saved on disk.
type record struct {
	ID       string    `json:"id"`
	Created  time.Time `json:"created"`
	Dir      string    `json:"dir"`
	Command  []string  `json:"command,omitempty"`
	Relaunch bool      `json:"relaunch,omitempty"`
	Restart  []string  `json:"restart,omitempty"` // nil with Relaunch: the login shell
	Exited   bool      `json:"exited,omitempty"`
	ExitCode int       `json:"exitCode,omitempty"`
	Name     string    `json:"name,omitempty"`
	Prompt   string    `json:"prompt,omitempty"`
	Project  string    `json:"project,omitempty"`
	Agent    string    `json:"agent,omitempty"`
	Branch   string    `json:"branch,omitempty"`
	Worktree bool      `json:"worktree,omitempty"`
	Skip     bool      `json:"skipPermissions,omitempty"`
	Budget   int64     `json:"budget,omitempty"` // seconds
	Display  bool      `json:"display,omitempty"`
	Parent   string    `json:"parent,omitempty"`
}

// persistence saves the daemon's sessions. Its methods are safe from any
// goroutine; writes are serialized by mu.
type persistence struct {
	dir string

	mu      sync.Mutex
	records map[string]record
	order   []string
	closing bool // the daemon is stopping: exits are not the user's doing
}

func openPersistence(dir string) (*persistence, error) {
	p := &persistence{dir: dir, records: map[string]record{}}
	data, err := os.ReadFile(p.file())
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daemon: sessions: %w", err)
	}
	var list []record
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("daemon: read %s: %w", p.file(), err)
	}
	for _, r := range list {
		p.records[r.ID] = r
		p.order = append(p.order, r.ID)
	}
	return p, nil
}

func (p *persistence) file() string { return filepath.Join(p.dir, "sessions.json") }

func (p *persistence) historyFile(id string) string {
	return filepath.Join(p.dir, "history", id+".vt")
}

// saved returns the records in creation order.
func (p *persistence) saved() []record {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]record, 0, len(p.order))
	for _, id := range p.order {
		out = append(out, p.records[id])
	}
	return out
}

// update records the session's current state when it changed in a way
// that matters after a restart.
func (p *persistence) update(s *session.Session, command []string) {
	info, meta := s.Info(), s.Meta()
	if meta.Queued {
		return // nothing runs yet: a restart forgets it
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closing {
		return
	}
	old, known := p.records[info.ID]
	r := record{
		ID: info.ID, Created: info.Created, Dir: info.Dir, Command: info.Command, Relaunch: meta.Relaunch, Restart: meta.Restart,
		Exited: info.Exited, ExitCode: info.ExitCode, Name: meta.Name, Prompt: info.Prompt,
		Project: meta.Project, Agent: meta.Agent, Branch: meta.Branch, Worktree: meta.Worktree, Skip: meta.SkipPermissions,
		Budget: int64(meta.Budget / time.Second), Display: meta.Display > 0, Parent: meta.Parent,
	}
	if command != nil {
		r.Command = command
	} else if known {
		r.Command = old.Command
	}
	if known && equalRecords(old, r) {
		return
	}
	p.records[r.ID] = r
	if !known {
		p.order = append(p.order, r.ID)
	}
	p.saveLocked()
}

func (p *persistence) remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.records[id]; !ok {
		return
	}
	delete(p.records, id)
	for i, o := range p.order {
		if o == id {
			p.order = append(p.order[:i], p.order[i+1:]...)
			break
		}
	}
	p.saveLocked()
	_ = os.Remove(p.historyFile(id))
}

// saveHistory writes what the session shows, keeping its last part.
func (p *persistence) saveHistory(s *session.Session) {
	h := s.History()
	if len(h) > historyMaxBytes {
		h = h[len(h)-historyMaxBytes:]
		if i := bytes.IndexByte(h, '\n'); i >= 0 {
			h = h[i+1:] // start on a whole row
		}
	}
	path := p.historyFile(s.ID())
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = writeFileAtomic(path, h)
}

func (p *persistence) loadHistory(id string) []byte {
	h, _ := os.ReadFile(p.historyFile(id))
	return h
}

// stopping marks the daemon as stopping: the sessions it hangs up next are
// relaunched at the next start, not recorded as exited.
func (p *persistence) stopping() {
	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
}

func (p *persistence) saveLocked() {
	list := make([]record, 0, len(p.order))
	for _, id := range p.order {
		list = append(list, p.records[id])
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(p.dir, 0o700); err != nil {
		return
	}
	_ = writeFileAtomic(p.file(), data)
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func equalRecords(a, b record) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

// restore brings back the saved sessions: running ones relaunched (agents
// continue their last conversation), the others as they ended.
func (s *Server) restore() {
	for _, r := range s.persist.saved() {
		spec := session.Spec{
			ID: r.ID, Dir: r.Dir, Created: r.Created, History: s.persist.loadHistory(r.ID), Env: s.sessionEnv(),
			Meta: session.Meta{
				Name: r.Name, Prompt: r.Prompt, Project: r.Project, Agent: r.Agent, Branch: r.Branch,
				Worktree: r.Worktree, SkipPermissions: r.Skip, Relaunch: r.Relaunch, Restart: r.Restart, Restored: true,
				Budget: time.Duration(r.Budget) * time.Second, Parent: r.Parent,
			},
		}
		_, dirErr := os.Stat(r.Dir)
		if r.Exited || !r.Relaunch || dirErr != nil {
			s.exitsLogged.Store(r.ID, true) // ended with an earlier daemon
			spec.Exited, spec.ExitCode, spec.Command = true, r.ExitCode, r.Command
			if !r.Exited {
				spec.ExitCode = -1 // stopped with the daemon, not relaunched
			}
		} else {
			spec.Command = r.Restart
			if preset, ok := agent.Find(r.Agent); ok {
				spec.Env = append(spec.Env, preset.Environ(r.Skip)...)
			}
			if preset, ok := agent.Find(r.Agent); ok && len(r.Restart) > 0 && len(preset.Continue) > 0 {
				// The command saved has the hooks and guide of the daemon
				// that saved it.
				spec.Command = thenShell(preset.Argv(agentLaunch(preset, r.Skip, "", true)))
				spec.Meta.Restart = spec.Command
			}
			if r.Display {
				// A new display: the old one went with the daemon.
				if disp, err := s.displays.start(r.ID); err == nil {
					spec.Env = append(spec.Env, disp.Env()...)
					spec.Meta.Display, spec.Meta.DisplayScale = disp.Port, disp.Scale
				} else {
					s.logger().Warn("could not start a session's display", "id", r.ID, "err", err)
				}
			}
		}
		sess, err := s.sessions.Create(spec)
		if err != nil {
			s.logger().Warn("could not restore a session", "id", r.ID, "err", err)
			continue
		}
		s.persist.update(sess, r.Command)
	}
}

// saveHistories saves what every session shows; it runs until done closes.
func (s *Server) saveHistories(done <-chan struct{}) {
	t := time.NewTicker(historySaveEvery)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		for _, sess := range s.sessions.List() {
			s.persist.saveHistory(sess)
		}
	}
}

// continueSession starts session id's program again in the session, its
// screen kept: an agent picks up its last conversation, a shell starts
// over. It is for sessions whose program ended, by itself or killed.
func (s *Server) continueSession(id string) (*session.Session, error) {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return nil, err
	}
	info, meta := sess.Info(), sess.Meta()
	if !info.Exited {
		return nil, errors.New("the session is still running")
	}
	if !meta.Relaunch {
		return nil, errors.New("this session's command can't be continued: start it again in a new session")
	}
	command := meta.Restart
	if preset, ok := agent.Find(meta.Agent); ok && len(preset.Continue) > 0 {
		command = thenShell(preset.Argv(agentLaunch(preset, meta.SkipPermissions, "", true)))
	}
	meta.Restart, meta.Restored = command, false
	spec := session.Spec{ID: id, Dir: info.Dir, Created: info.Created, History: sess.History(),
		Env: s.sessionEnv(), Command: command, Meta: meta}
	if meta.Display > 0 {
		// Its display went with its program, or stays: one either way.
		s.displays.stop(id)
		meta.Display, meta.DisplayScale = 0, 0
		if disp, err := s.displays.start(id); err == nil {
			spec.Env = append(spec.Env, disp.Env()...)
			meta.Display, meta.DisplayScale = disp.Port, disp.Scale
		}
		spec.Meta = meta
	}
	next, err := s.sessions.Replace(spec)
	if err != nil {
		return nil, err
	}
	s.began(next, command, "")
	return next, nil
}
