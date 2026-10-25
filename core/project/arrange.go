package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// How the user arranges projects: their order is the order of the paths in
// the store's file; their groups are in a file beside it, so a daemon of
// before groups still reads the list.

// groupsPath is the file of the projects' groups, beside the list.
func (s *Store) groupsPath() string {
	return strings.TrimSuffix(s.path, filepath.Ext(s.path)) + "-groups.json"
}

func (s *Store) loadGroups() error {
	data, err := os.ReadFile(s.groupsPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("project: %w", err)
	}
	if err := json.Unmarshal(data, &s.groups); err != nil {
		return fmt.Errorf("project: read %s: %w", s.groupsPath(), err)
	}
	return nil
}

// saveGroups writes the groups; s.mu is held.
func (s *Store) saveGroups() error {
	data, err := json.MarshalIndent(s.groups, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return writeAtomic(s.groupsPath(), data)
}

// Arrange puts the project at path in group ("" for none) and moves it
// before the project at before, or to the end with before "".
func (s *Store) Arrange(path, group, before string) error {
	group = strings.TrimSpace(group)
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.Index(s.paths, path)
	if i < 0 {
		return fmt.Errorf("project: %s is not a project", path)
	}
	if before != "" && !slices.Contains(s.paths, before) {
		return fmt.Errorf("project: %s is not a project", before)
	}
	if before != path {
		paths := slices.Delete(slices.Clone(s.paths), i, i+1)
		at := len(paths)
		if before != "" {
			at = slices.Index(paths, before)
		}
		s.paths = slices.Insert(paths, at, path)
	}
	s.setGroup(path, group)
	if err := s.saveGroups(); err != nil {
		return err
	}
	return s.save()
}

// MoveGroup moves group's projects, in their order, before the project at
// before (of another group or none), or to the end with before "".
func (s *Store) MoveGroup(group, before string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var moved, rest []string
	for _, p := range s.paths {
		if s.groups[p] == group {
			moved = append(moved, p)
		} else {
			rest = append(rest, p)
		}
	}
	if group == "" || len(moved) == 0 {
		return fmt.Errorf("project: no group %q", group)
	}
	at := len(rest)
	if before != "" {
		if at = slices.Index(rest, before); at < 0 {
			return fmt.Errorf("project: %s is not a project outside %q", before, group)
		}
	}
	s.paths = slices.Insert(rest, at, moved...)
	return s.save()
}

// RenameGroup renames a group; renaming it "" ungroups its projects, and
// to a group there already merges them into it.
func (s *Store) RenameGroup(from, to string) error {
	to = strings.TrimSpace(to)
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for p, g := range s.groups {
		if g == from {
			s.setGroup(p, to)
			found = true
		}
	}
	if !found {
		return fmt.Errorf("project: no group %q", from)
	}
	return s.saveGroups()
}

// setGroup sets one project's group; s.mu is held.
func (s *Store) setGroup(path, group string) {
	if group == "" {
		delete(s.groups, path)
		return
	}
	if s.groups == nil {
		s.groups = map[string]string{}
	}
	s.groups[path] = group
}
