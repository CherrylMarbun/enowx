package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/core/gitclone"
	"github.com/enowx/enowx/proto"
)

// clones are the host's clones, running and done lately, which the app
// and agents follow.
type clones struct {
	mu   sync.Mutex
	list []*cloneJob // oldest first; the last few done are kept
}

type cloneJob struct {
	status proto.CloneStatus // guarded by clones.mu
	cancel context.CancelFunc
}

// keepDone is how many finished clones are kept to be asked about.
const keepDone = 10

// startClone starts cloning p, returning its first status.
func (s *Server) startClone(p proto.ProjectCloneParams) (proto.CloneStatus, error) {
	if err := gitclone.Valid(p.URL); err != nil {
		return proto.CloneStatus{}, err
	}
	dir := strings.TrimSpace(p.Dir)
	if dir == "" {
		return proto.CloneStatus{}, errors.New("pick the folder to clone into")
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return proto.CloneStatus{}, err
		}
		dir = filepath.Join(home, dir[1:])
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = gitclone.Name(p.URL)
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return proto.CloneStatus{}, fmt.Errorf("%q can't be a folder's name", name)
	}
	path := filepath.Join(dir, name)
	if entries, err := os.ReadDir(path); err == nil && len(entries) > 0 {
		return proto.CloneStatus{}, fmt.Errorf("%s exists and isn't empty: pick another folder or name", path)
	}
	var b [6]byte
	rand.Read(b[:])
	ctx, cancel := context.WithCancel(context.Background())
	job := &cloneJob{cancel: cancel, status: proto.CloneStatus{
		ID: hex.EncodeToString(b[:]), URL: gitclone.Redact(p.URL), Path: path, State: proto.CloneRunning, Phase: "Starting",
	}}
	s.clones.mu.Lock()
	s.clones.list = append(s.clones.list, job)
	first := job.status
	s.clones.mu.Unlock()
	s.broadcast(proto.EventProjectClone, first)

	go func() {
		defer cancel()
		var sent time.Time
		err := gitclone.Clone(ctx, gitclone.Options{URL: p.URL, Path: path, Branch: p.Branch, Depth: p.Depth, Submodules: p.Submodules}, func(pr gitclone.Progress) {
			s.clones.mu.Lock()
			st := &job.status
			changed := pr.Phase != "" && (pr.Phase != st.Phase || pr.Percent != st.Percent)
			if pr.Phase != "" {
				st.Phase, st.Percent = pr.Phase, pr.Percent
			}
			st.Line = pr.Line
			now := *st
			s.clones.mu.Unlock()
			// git says a lot: what changes, and a few times a second.
			if changed && time.Since(sent) > 150*time.Millisecond {
				sent = time.Now()
				s.broadcast(proto.EventProjectClone, now)
			}
		})
		var info proto.ProjectInfo
		if err == nil {
			info, err = s.projects.Add(path)
		}
		s.clones.mu.Lock()
		st := &job.status
		switch {
		case errors.Is(err, context.Canceled):
			st.State, st.Error = proto.CloneCanceled, ""
		case err != nil:
			st.State, st.Error = proto.CloneFailed, err.Error()
		default:
			st.State, st.Percent, st.Project = proto.CloneDone, 100, &info
		}
		last := *st
		s.clones.trim()
		s.clones.mu.Unlock()
		if err == nil {
			s.broadcast(proto.EventProjectsChanged, s.projects.List())
		}
		s.broadcast(proto.EventProjectClone, last)
	}()
	return first, nil
}

// trim forgets the oldest finished clones past keepDone; mu is held.
func (c *clones) trim() {
	done := 0
	for i := len(c.list) - 1; i >= 0; i-- {
		if c.list[i].status.State == proto.CloneRunning {
			continue
		}
		if done++; done > keepDone {
			c.list = append(c.list[:i], c.list[i+1:]...)
		}
	}
}

func (s *Server) cloneStatus(id string) (proto.CloneStatus, error) {
	s.clones.mu.Lock()
	defer s.clones.mu.Unlock()
	for _, j := range s.clones.list {
		if j.status.ID == id {
			return j.status, nil
		}
	}
	return proto.CloneStatus{}, fmt.Errorf("no clone %q", id)
}

func (s *Server) cancelClone(id string) error {
	s.clones.mu.Lock()
	defer s.clones.mu.Unlock()
	for _, j := range s.clones.list {
		if j.status.ID == id {
			j.cancel()
			return nil
		}
	}
	return fmt.Errorf("no clone %q", id)
}

func (s *Server) cloneList() []proto.CloneStatus {
	s.clones.mu.Lock()
	defer s.clones.mu.Unlock()
	out := []proto.CloneStatus{}
	for _, j := range s.clones.list {
		out = append(out, j.status)
	}
	return out
}

// stopAll cancels the clones running.
func (c *clones) stopAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, j := range c.list {
		j.cancel()
	}
}
