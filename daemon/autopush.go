package daemon

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/enowx/enowx/core/autopush"
	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/proto"
)

// Auto pushes run here, with no app needed, for the projects whose
// settings name this daemon as the one pushing: one host per project, so
// two never push over each other.

// autopushTick is how often the daemon looks for auto pushes due.
const autopushTick = 20 * time.Second

// autopushes keeps which projects push now, so one runs at a time each.
type autopushes struct {
	mu      sync.Mutex
	running map[string]bool
	// Env adds credentials to git's environment for a project's push
	// (the vault's, when it holds them); nil uses the host's git setup.
	Env func(dir string) []string
}

func (a *autopushes) start(project string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running[project] {
		return false
	}
	if a.running == nil {
		a.running = map[string]bool{}
	}
	a.running[project] = true
	return true
}

func (a *autopushes) done(project string) {
	a.mu.Lock()
	delete(a.running, project)
	a.mu.Unlock()
}

// runAutopushes pushes the projects due, until done closes.
func (s *Server) runAutopushes(done <-chan struct{}) {
	t := time.NewTicker(autopushTick)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
		}
		list, err := s.links.Autopushes()
		if err != nil {
			continue
		}
		states := s.links.PushStates()
		for _, a := range list {
			if !a.On || a.Host != s.daemonID {
				continue
			}
			if time.Since(states[a.Project].LastRun) < projectlinks.Interval(a) {
				continue
			}
			dir := s.projectByID(a.Project)
			if dir == "" {
				continue
			}
			go func() { _, _ = s.autopushRun(a, dir) }()
		}
	}
}

// autopushRun snapshots the project at dir and pushes it when it changed
// since the last push, records how it went, and tells the apps.
func (s *Server) autopushRun(a proto.AutopushSettings, dir string) (proto.AutopushStatus, error) {
	if !s.pushes.start(a.Project) {
		return s.autopushStatus(a), errors.New("autopush: already pushing")
	}
	defer s.pushes.done(a.Project)
	st := s.links.PushStates()[a.Project]
	st.LastRun, st.Err = time.Now().UTC(), ""
	env := s.vaultGitEnv()
	if s.pushes.Env != nil {
		env = s.pushes.Env(dir)
	}
	host, _ := os.Hostname()
	if a.HostName != "" {
		host = a.HostName
	}
	snap, err := autopush.Take(dir, autopush.Options{Branch: a.Branch, Host: host, Env: env})
	summary := ""
	if err == nil {
		st.Commit, st.Files = snap.Commit, snap.Files
		if snap.Commit != "" && snap.Commit != st.Pushed {
			if err = autopush.Push(dir, a.Remote, a.Branch, env); err == nil {
				st.Pushed, st.LastPush = snap.Commit, st.LastRun
				summary = fmt.Sprintf("pushed %s to %s (%d files)", short(snap.Commit), a.Branch, snap.Files)
				if a.PR {
					if url, perr := autopush.OpenPullRequest(dir, a.Branch, a.Base); perr == nil {
						st.PRURL = url
					} else {
						summary += "; " + perr.Error()
					}
				}
			}
		} else {
			summary = "nothing new to push"
		}
		if len(snap.Skipped) > 0 && err == nil {
			summary += fmt.Sprintf("; left out %d files of secrets", len(snap.Skipped))
		}
	}
	if err != nil {
		st.Err = err.Error()
	}
	_ = s.links.SetPushState(a.Project, st)
	entry := proto.ProjectHistoryEntry{Time: st.LastRun, Project: a.Project, Kind: "push", OK: err == nil, Summary: summary}
	if err != nil {
		entry.Err = err.Error()
	}
	_ = s.links.AddHistory(entry)
	status := s.autopushStatus(a)
	s.broadcast(proto.EventAutopush, status)
	s.broadcast(proto.EventProjectHistory, entry)
	return status, err
}

func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// autopushStatus is a's settings with how its pushes went here.
func (s *Server) autopushStatus(a proto.AutopushSettings) proto.AutopushStatus {
	st := s.links.PushStates()[a.Project]
	out := proto.AutopushStatus{AutopushSettings: a, Here: a.Host == s.daemonID, LastRun: st.LastRun, LastPush: st.LastPush,
		Commit: st.Commit, Files: st.Files, Err: st.Err, Visibility: st.Visibility, PRURL: st.PRURL}
	if out.Branch == "" {
		out.Branch = autopush.DefaultBranch
	}
	if dir := s.projectByID(a.Project); dir != "" {
		out.Path = dir
		out.BranchURL = autopush.BranchLink(autopush.RemoteURL(dir, a.Remote), out.Branch)
	}
	if out.Here && a.On {
		out.Next = st.LastRun.Add(projectlinks.Interval(a))
		if st.LastRun.IsZero() {
			out.Next = time.Now().UTC().Add(autopushTick)
		}
	}
	return out
}

// autopushList is every project's auto pushes.
func (s *Server) autopushList() ([]proto.AutopushStatus, error) {
	list, err := s.links.Autopushes()
	if err != nil {
		return nil, err
	}
	out := []proto.AutopushStatus{}
	for _, a := range list {
		out = append(out, s.autopushStatus(a))
	}
	return out, nil
}

// autopushSet records a project's auto push settings, a.Path being its
// folder here; turned on without a host, this host pushes it.
func (s *Server) autopushSet(a proto.AutopushSettings) (proto.AutopushStatus, error) {
	dir := ""
	if a.Path != "" {
		id, err := s.projectID(a.Path)
		if err != nil {
			return proto.AutopushStatus{}, err
		}
		dir, a.Project = id.Path, id.ID
	} else {
		dir = s.projectByID(a.Project)
	}
	if !projectlinks.ValidID(a.Project) {
		return proto.AutopushStatus{}, errors.New("autopush: which project? its folder here or its ID")
	}
	if a.Branch == "" {
		a.Branch = autopush.DefaultBranch
	}
	if a.On && a.Host == "" {
		a.Host = s.daemonID
	}
	if a.Host == s.daemonID {
		a.HostName, _ = os.Hostname()
		if a.On {
			if dir == "" {
				return proto.AutopushStatus{}, errors.New("autopush: the project isn't on this host")
			}
			if _, err := git.TopLevel(dir); err != nil {
				return proto.AutopushStatus{}, errors.New("autopush: the project isn't a git repository")
			}
			if err := autopush.CheckBranch(dir, a.Branch); err != nil {
				return proto.AutopushStatus{}, err
			}
			st := s.links.PushStates()[a.Project]
			st.Visibility = autopush.Visibility(dir)
			_ = s.links.SetPushState(a.Project, st)
		}
	}
	if err := s.links.SetAutopush(a); err != nil {
		return proto.AutopushStatus{}, err
	}
	status := s.autopushStatus(a)
	s.broadcast(proto.EventAutopush, status)
	return status, nil
}

// autopushNow pushes the project at path now, by its settings (or the
// defaults), when this host is the one pushing it.
func (s *Server) autopushNow(path string) (proto.AutopushStatus, error) {
	id, err := s.projectID(path)
	if err != nil {
		return proto.AutopushStatus{}, err
	}
	a, found, err := s.links.Autopush(id.ID)
	if err != nil {
		return proto.AutopushStatus{}, err
	}
	if found && a.Host != "" && a.Host != s.daemonID {
		return proto.AutopushStatus{}, fmt.Errorf("autopush: %s pushes this project: push from there", a.HostName)
	}
	if !found {
		a = proto.AutopushSettings{Project: id.ID, Branch: autopush.DefaultBranch}
	}
	return s.autopushRun(a, id.Path)
}
