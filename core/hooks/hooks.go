// Package hooks keeps the user's hooks (what to run when something
// happens in a session, proto.Hook), says which match an event and runs
// them. The host's own hooks are in settings/hooks.json, synced to every
// host; a project's in its .enowx/hooks.json, run only once the user
// allowed that file as it is. See docs/hooks.md.
package hooks

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/proto"
)

// ProjectFile is where a project keeps its hooks, from its folder.
const ProjectFile = ".enowx/hooks.json"

// Store is a host's hooks and the projects' hook files the user allowed.
// Safe from any goroutine.
type Store struct {
	file, approvals string

	mu      sync.Mutex
	list    []proto.Hook
	stamp   fileStamp // of file when list was read, to see a sync's change
	allowed map[string]string
}

type fileStamp struct {
	mod  int64
	size int64
}

// Open reads the hooks kept in dir: dir/settings/hooks.json, and the
// approvals in dir/hooks-allowed.json (per host: a folder means nothing
// on another).
func Open(dir string) *Store {
	s := &Store{file: filepath.Join(dir, "settings", "hooks.json"), approvals: filepath.Join(dir, "hooks-allowed.json"), allowed: map[string]string{}}
	s.reload()
	if data, err := os.ReadFile(s.approvals); err == nil {
		_ = json.Unmarshal(data, &s.allowed)
	}
	return s
}

func stampOf(path string) fileStamp {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{mod: fi.ModTime().UnixNano(), size: fi.Size()}
}

// reload reads the file again when it changed (a sync brought another
// host's edit). s.mu is not held.
func (s *Store) reload() {
	st := stampOf(s.file)
	s.mu.Lock()
	defer s.mu.Unlock()
	if st == s.stamp && s.list != nil {
		return
	}
	var list []proto.Hook
	if data, err := os.ReadFile(s.file); err == nil {
		_ = json.Unmarshal(data, &list)
	}
	s.list, s.stamp = valid(list), st
}

// List is the host's own hooks, in order.
func (s *Store) List() []proto.Hook {
	s.reload()
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.list)
}

// Save replaces the host's hooks with list, giving new ones an ID.
func (s *Store) Save(list []proto.Hook) ([]proto.Hook, error) {
	for i := range list {
		if err := Check(list[i]); err != nil {
			return nil, err
		}
		if list[i].ID == "" {
			list[i].ID = newID()
		}
		list[i].Project = ""
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(s.file), 0o700); err != nil {
		return nil, fmt.Errorf("hooks: %w", err)
	}
	if err := envsync.WriteFileAtomic(s.file, data, 0o600); err != nil {
		return nil, fmt.Errorf("hooks: save: %w", err)
	}
	s.mu.Lock()
	s.list, s.stamp = slices.Clone(list), stampOf(s.file)
	s.mu.Unlock()
	return list, nil
}

// Check says what is wrong with hook h, nil when nothing.
func Check(h proto.Hook) error {
	if !slices.Contains(proto.HookEvents, h.Event) {
		return fmt.Errorf("hooks: unknown event %q", h.Event)
	}
	switch h.Action {
	case proto.HookActionRun:
		if h.Command == "" {
			return errors.New("hooks: a hook that runs a command needs the command")
		}
	case proto.HookActionContext:
		if h.Context == "" {
			return errors.New("hooks: a hook that adds context needs the text")
		}
	case proto.HookActionBlock:
	default:
		return fmt.Errorf("hooks: unknown action %q", h.Action)
	}
	if _, err := compile(h.Tool); err != nil {
		return fmt.Errorf("hooks: tool: %w", err)
	}
	if _, err := compile(h.Text); err != nil {
		return fmt.Errorf("hooks: text: %w", err)
	}
	return nil
}

// valid keeps the hooks of list that Check passes: a hand-edited file
// with one bad hook still runs the others.
func valid(list []proto.Hook) []proto.Hook {
	out := []proto.Hook{}
	for _, h := range list {
		if Check(h) == nil {
			out = append(out, h)
		}
	}
	return out
}

// Project reads the hooks of the project in dir: its hooks, the file's
// hash, and whether the user allowed the file as it is. No file, no
// hooks.
func (s *Store) Project(dir string) (proto.ProjectHooks, bool) {
	data, err := os.ReadFile(filepath.Join(dir, ProjectFile))
	if err != nil {
		return proto.ProjectHooks{Dir: dir}, false
	}
	sum := sha256.Sum256(data)
	ph := proto.ProjectHooks{Dir: dir, Hash: hex.EncodeToString(sum[:])}
	var list []proto.Hook
	if err := json.Unmarshal(data, &list); err != nil {
		var wrapped struct {
			Hooks []proto.Hook `json:"hooks"`
		}
		_ = json.Unmarshal(data, &wrapped)
		list = wrapped.Hooks
	}
	for i, h := range valid(list) {
		if h.ID == "" {
			h.ID = fmt.Sprintf("%s#%d", ph.Hash[:8], i)
		}
		h.Project = dir
		ph.Hooks = append(ph.Hooks, h)
	}
	s.mu.Lock()
	allowed := s.allowed[dir] == ph.Hash
	s.mu.Unlock()
	return ph, allowed
}

// Allow allows, or stops allowing, the hooks of project dir's file with
// hash; a file changed since asks again.
func (s *Store) Allow(dir, hash string, allow bool) error {
	s.mu.Lock()
	if allow {
		s.allowed[dir] = hash
	} else {
		delete(s.allowed, dir)
	}
	data, err := json.MarshalIndent(s.allowed, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := envsync.WriteFileAtomic(s.approvals, data, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("hooks: allow: %w", err)
	}
	return nil
}

func newID() string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
