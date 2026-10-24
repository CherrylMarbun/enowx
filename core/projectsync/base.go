package projectsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/enowx/enowx/proto"
)

// Base is what both copies had at the last sync: what a side holds that
// differs from it changed there since.
type Base struct {
	// Files are hashes by path, by folder: "" is the project's, a session
	// key a worktree session's.
	Files    map[string]map[string]string `json:"files,omitempty"`
	Branches map[string]string            `json:"branches,omitempty"`
	// Sessions are the sessions both had, with their conversation's hash.
	Sessions map[string]string `json:"sessions,omitempty"`
}

// BaseOf is what here and there have alike, scanned after a sync.
func BaseOf(here, there proto.ProjectSyncState) Base {
	b := Base{Files: map[string]map[string]string{"": sameFiles(here.Files, there.Files)}, Branches: map[string]string{}, Sessions: map[string]string{}}
	for n, c := range here.Branches {
		if there.Branches[n] == c {
			b.Branches[n] = c
		}
	}
	t := keyed(there.Sessions)
	for _, h := range here.Sessions {
		o, ok := t[h.Key]
		if !ok {
			continue
		}
		talk := ""
		if h.Talk == o.Talk {
			talk = h.Talk
		}
		b.Sessions[h.Key] = talk
		if h.Worktree && o.Worktree {
			b.Files[h.Key] = sameFiles(h.Files, o.Files)
		}
	}
	return b
}

func sameFiles(a, b []proto.SyncFile) map[string]string {
	out := map[string]string{}
	bp := filesByPath(b)
	for _, f := range a {
		if o, ok := bp[f.Path]; ok && o.Hash == f.Hash {
			out[f.Path] = f.Hash
		}
	}
	return out
}

// End is one copy of a project: a host (by its id in the device's server
// list) and the project's folder there.
type End struct {
	Host string `json:"host"`
	Path string `json:"path"`
}

// Pair is two copies of a project synced before, and their base.
type Pair struct {
	Ends [2]End `json:"ends"`
	Base Base   `json:"base"`
}

// Store keeps the pairs synced on this device, in one file. It is safe
// for use by several goroutines.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore keeps pairs in the file at path.
func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) load() ([]Pair, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("projectsync: %w", err)
	}
	var pairs []Pair
	if err := json.Unmarshal(data, &pairs); err != nil {
		return nil, fmt.Errorf("projectsync: %s: %w", s.path, err)
	}
	return pairs, nil
}

// Partners lists the other copies of the project at end, last synced
// first.
func (s *Store) Partners(end End) []End {
	s.mu.Lock()
	defer s.mu.Unlock()
	pairs, _ := s.load()
	var out []End
	for i := len(pairs) - 1; i >= 0; i-- {
		for side, e := range pairs[i].Ends {
			if e == end {
				out = append(out, pairs[i].Ends[1-side])
			}
		}
	}
	return out
}

// Base is the base of the pair a and b, nil when they never synced.
func (s *Store) Base(a, b End) *Base {
	s.mu.Lock()
	defer s.mu.Unlock()
	pairs, _ := s.load()
	if i := find(pairs, a, b); i >= 0 {
		return &pairs[i].Base
	}
	return nil
}

// Save records the base of the pair a and b, as the last synced.
func (s *Store) Save(a, b End, base Base) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pairs, err := s.load()
	if err != nil {
		return err
	}
	if i := find(pairs, a, b); i >= 0 {
		pairs = slices.Delete(pairs, i, i+1)
	}
	pairs = append(pairs, Pair{Ends: [2]End{a, b}, Base: base})
	data, err := json.MarshalIndent(pairs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("projectsync: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("projectsync: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func find(pairs []Pair, a, b End) int {
	return slices.IndexFunc(pairs, func(p Pair) bool {
		return p.Ends == [2]End{a, b} || p.Ends == [2]End{b, a}
	})
}
