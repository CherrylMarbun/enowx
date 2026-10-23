package commands

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

// maxRecent is how many commands run lately a project keeps.
const maxRecent = 6

// project is what the store keeps of a project.
type project struct {
	Saved  []proto.RunCommand `json:"saved,omitempty"`
	Recent []proto.RunCommand `json:"recent,omitempty"`
}

// Store keeps the commands saved and run lately per project, as JSON in
// one file on the daemon's host: the project's own files stay untouched.
// It is safe for concurrent use.
type Store struct {
	path string

	mu        sync.Mutex
	byProject map[string]*project
}

// OpenStore loads the store at path, which may not exist yet.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, byProject: map[string]*project{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("commands: %w", err)
	}
	if err := json.Unmarshal(data, &s.byProject); err != nil {
		return nil, fmt.Errorf("commands: read %s: %w", path, err)
	}
	return s, nil
}

// For lists the commands of key's project, found in dir: saved first,
// then those run lately, then those its files name, each command once.
func (s *Store) For(key, dir string) []proto.RunCommand {
	s.mu.Lock()
	var saved, recent []proto.RunCommand
	if p := s.byProject[key]; p != nil {
		saved, recent = slices.Clone(p.Saved), slices.Clone(p.Recent)
	}
	s.mu.Unlock()
	var out []proto.RunCommand
	seen := map[string]bool{}
	add := func(list []proto.RunCommand, source string) {
		for _, c := range list {
			if seen[c.Command] {
				continue
			}
			seen[c.Command] = true
			if source != "" {
				c.Source = source
			}
			out = append(out, c)
		}
	}
	add(saved, proto.CommandSaved)
	add(recent, proto.CommandRecent)
	add(Detect(dir), "")
	return out
}

// Save saves c for key's project, or replaces the saved one with its
// command.
func (s *Store) Save(key string, c proto.RunCommand) error {
	c, ok := clean(c)
	if !ok {
		return errors.New("commands: the command is empty")
	}
	return s.change(key, func(p *project) {
		p.Saved = append(drop(p.Saved, c.Command), c)
	})
}

// Remove forgets command, saved or recent, for key's project.
func (s *Store) Remove(key, command string) error {
	command = strings.TrimSpace(command)
	return s.change(key, func(p *project) {
		p.Saved, p.Recent = drop(p.Saved, command), drop(p.Recent, command)
	})
}

// Ran remembers that c ran in key's project, first among its recent
// commands, unless it is saved.
func (s *Store) Ran(key string, c proto.RunCommand) error {
	c, ok := clean(c)
	if !ok {
		return nil
	}
	return s.change(key, func(p *project) {
		if slices.ContainsFunc(p.Saved, func(o proto.RunCommand) bool { return o.Command == c.Command }) {
			return
		}
		p.Recent = append([]proto.RunCommand{c}, drop(p.Recent, c.Command)...)
		if len(p.Recent) > maxRecent {
			p.Recent = p.Recent[:maxRecent]
		}
	})
}

// clean trims c and names it after its command when it has no name.
func clean(c proto.RunCommand) (proto.RunCommand, bool) {
	c.Name, c.Command, c.Source = strings.TrimSpace(c.Name), strings.TrimSpace(c.Command), ""
	if c.Command == "" {
		return c, false
	}
	if c.Name == "" {
		c.Name = c.Command
	}
	return c, true
}

func drop(list []proto.RunCommand, command string) []proto.RunCommand {
	return slices.DeleteFunc(slices.Clone(list), func(c proto.RunCommand) bool { return c.Command == command })
}

// change applies fn to key's project and saves the store.
func (s *Store) change(key string, fn func(*project)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.byProject[key]
	if p == nil {
		p = &project{}
		s.byProject[key] = p
	}
	fn(p)
	if len(p.Saved) == 0 && len(p.Recent) == 0 {
		delete(s.byProject, key)
	}
	data, err := json.MarshalIndent(s.byProject, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("commands: %w", err)
	}
	return os.Rename(tmp, s.path)
}
