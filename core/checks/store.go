package checks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/enowx/enowx/proto"
)

// Store keeps the checks users set per project, as JSON in one file on
// the daemon's host: the project's own files stay untouched.
type Store struct {
	path string

	mu        sync.Mutex
	byProject map[string][]proto.Check
}

// OpenStore loads the store at path, which may not exist yet.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, byProject: map[string][]proto.Check{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checks: %w", err)
	}
	if err := json.Unmarshal(data, &s.byProject); err != nil {
		return nil, fmt.Errorf("checks: read %s: %w", path, err)
	}
	return s, nil
}

// For returns the checks of project, run in dir: those set for it, else
// those found in dir. custom says they were set.
func (s *Store) For(project, dir string) (checks []proto.Check, custom bool) {
	s.mu.Lock()
	set, ok := s.byProject[project]
	s.mu.Unlock()
	if ok && project != "" {
		return slices.Clone(set), true
	}
	return Detect(dir), false
}

// Set sets project's checks; none goes back to those found in its files.
func (s *Store) Set(project string, checks []proto.Check) error {
	var clean []proto.Check
	for _, c := range checks {
		c.Name, c.Command = strings.TrimSpace(c.Name), strings.TrimSpace(c.Command)
		if c.Command == "" {
			continue
		}
		if c.Name == "" {
			c.Name = strings.Fields(c.Command)[0]
		}
		clean = append(clean, c)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(clean) == 0 {
		delete(s.byProject, project)
	} else {
		s.byProject[project] = clean
	}
	data, err := json.MarshalIndent(s.byProject, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("checks: %w", err)
	}
	return os.Rename(tmp, s.path)
}
