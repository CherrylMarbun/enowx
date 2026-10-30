package daemon

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// What the daemon sees of a session from outside, for agents whose own
// hooks don't say it: its program starting and ending, a turn ending as
// its screen goes quiet; and, for every agent, the files a turn changed
// and the commits made in its folder.

// turnQuiet is how long the screen of an agent without hooks stays quiet
// before its turn counts as done.
const turnQuiet = 4 * time.Second

// commitLook is how often folders are looked at for a new commit, while a
// hook waits for one.
const commitLook = 5 * time.Second

// sessionWatch is what the daemon keeps of a session for its hooks.
// hookEngine.mu guards it.
type sessionWatch struct {
	started, ended bool
	working        bool
	quiet          *time.Timer
	dirty          map[string]fileStamp // the folder's changed files when its turn began; nil when no hook asks
}

type fileStamp struct {
	mod  int64
	size int64
}

func (e *hookEngine) watchOf(id string) *sessionWatch {
	w := e.watch[id]
	if w == nil {
		w = &sessionWatch{}
		e.watch[id] = w
	}
	return w
}

// wants says some hook that may run in folder dir is on for event.
func (s *Server) wants(dir, event string) bool {
	return slices.ContainsFunc(s.hooksFor(dir), func(h proto.Hook) bool { return h.On && h.Event == event })
}

// watchSession follows session info's updates for the events its agent's
// hooks don't hand over.
func (s *Server) watchSession(info proto.SessionInfo) {
	if info.NotesFor != "" || info.CanvasFor != "" || info.Parent != "" {
		return // the app's own agents, and tabs: not the user's sessions
	}
	support := agent.HookSupport(info.Agent)
	outside := func(event string) bool { return support[event] == proto.HookOutside }
	s.hooks.mu.Lock()
	w := s.hooks.watchOf(info.ID)
	start := !w.started && !info.Exited
	w.started = w.started || start
	end := info.Exited && !w.ended
	if end {
		w.ended = true
		if w.quiet != nil {
			w.quiet.Stop()
		}
		delete(s.hooks.watch, info.ID)
	}
	var began, quiet bool
	if outside(proto.HookTurnDone) && !info.Exited {
		working := info.State == proto.StateWorking || info.State == "" && info.Active
		began, quiet = working && !w.working, !working && w.working
		w.working = working
		if began && w.quiet != nil {
			w.quiet.Stop()
		}
	}
	s.hooks.mu.Unlock()

	if start && outside(proto.HookSessionStart) {
		go s.fireHooks(info, proto.HookEvent{Event: proto.HookSessionStart, Via: proto.HookOutside})
	}
	if began {
		go s.turnStarted(info)
	}
	if quiet {
		s.hooks.mu.Lock()
		if w := s.hooks.watch[info.ID]; w != nil {
			id := info.ID
			w.quiet = time.AfterFunc(turnQuiet, func() { s.quietTurn(id) })
		}
		s.hooks.mu.Unlock()
	}
	if end && outside(proto.HookSessionEnd) {
		go s.fireHooks(info, proto.HookEvent{Event: proto.HookSessionEnd, Via: proto.HookOutside})
	}
}

// quietTurn ends the turn of session id's agent, its screen quiet since.
func (s *Server) quietTurn(id string) {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return
	}
	info := sess.Info()
	s.hooks.mu.Lock()
	w := s.hooks.watch[id]
	still := w != nil && !w.working
	s.hooks.mu.Unlock()
	if !still || info.Exited {
		return
	}
	s.fireHooks(info, proto.HookEvent{Event: proto.HookTurnDone, Via: proto.HookOutside})
	s.turnEnded(info)
}

// turnStarted notes the files already changed in session info's folder,
// when a hook asks for those a turn changes.
func (s *Server) turnStarted(info proto.SessionInfo) {
	if !s.wants(info.Dir, proto.HookFileChanged) {
		return
	}
	snap := changedFiles(info.Dir)
	s.hooks.mu.Lock()
	s.hooks.watchOf(info.ID).dirty = snap
	s.hooks.mu.Unlock()
}

// turnEnded runs the hooks on the files session info's turn changed:
// those changed now that weren't, or changed again, since it began.
func (s *Server) turnEnded(info proto.SessionInfo) {
	s.hooks.mu.Lock()
	var before map[string]fileStamp
	if w := s.hooks.watch[info.ID]; w != nil {
		before, w.dirty = w.dirty, nil
	}
	s.hooks.mu.Unlock()
	if before == nil {
		return
	}
	now := changedFiles(info.Dir)
	var paths []string
	for p, st := range now {
		if old, ok := before[p]; !ok || old != st {
			paths = append(paths, p)
		}
	}
	for p := range before {
		if _, ok := now[p]; !ok {
			paths = append(paths, p) // committed or put back
		}
	}
	if len(paths) == 0 {
		return
	}
	slices.Sort(paths)
	s.fireHooks(info, proto.HookEvent{Event: proto.HookFileChanged, Paths: paths, Via: proto.HookOutside})
}

// changedFiles are the files git sees changed in folder dir, with their
// size and time; nil outside a repository.
func changedFiles(dir string) map[string]fileStamp {
	out, err := git.Run(dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil
	}
	files := map[string]fileStamp{}
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		p := e[3:]
		if e[0] == 'R' || e[0] == 'C' {
			i++ // the name it had follows
		}
		var st fileStamp
		if fi, err := os.Stat(filepath.Join(dir, p)); err == nil {
			st = fileStamp{mod: fi.ModTime().UnixNano(), size: fi.Size()}
		}
		files[p] = st
	}
	return files
}

// watchCommits looks for new commits in the folders of live sessions
// while a hook waits for one, until done closes.
func (s *Server) watchCommits(done <-chan struct{}) {
	heads := map[string]string{} // folder → commit last seen
	t := time.NewTicker(commitLook)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		seen := map[string]bool{}
		for _, sess := range s.sessions.List() {
			info := sess.Info()
			if info.Exited || info.Parent != "" || info.NotesFor != "" || info.CanvasFor != "" || seen[info.Dir] {
				continue
			}
			seen[info.Dir] = true
			if !s.wants(info.Dir, proto.HookCommit) {
				delete(heads, info.Dir)
				continue
			}
			head, err := git.Run(info.Dir, "rev-parse", "HEAD")
			if err != nil {
				continue
			}
			head = strings.TrimSpace(head)
			old, known := heads[info.Dir]
			heads[info.Dir] = head
			if !known || old == head {
				continue
			}
			subject, _ := git.Run(info.Dir, "log", "-1", "--format=%s", head)
			go s.fireHooks(info, proto.HookEvent{Event: proto.HookCommit, Commit: head, Message: strings.TrimSpace(subject), Via: proto.HookOutside})
		}
		for dir := range heads {
			if !seen[dir] {
				delete(heads, dir)
			}
		}
	}
}
