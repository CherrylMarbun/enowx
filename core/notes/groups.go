package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Groups of notes, like folders, in groups.json beside the notes: their
// order, icons, colors and whether they are folded. A top-level note says
// its group in its header; deleting a group leaves its notes in none.

func (s *Store) groupsFile() string { return filepath.Join(s.dir, "groups.json") }

// Groups is the groups, in the user's order.
func (s *Store) Groups() ([]proto.NoteGroup, error) {
	data, err := os.ReadFile(s.groupsFile())
	if errors.Is(err, os.ErrNotExist) {
		return []proto.NoteGroup{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notes: %w", err)
	}
	out := []proto.NoteGroup{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("notes: groups: %w", err)
	}
	return out, nil
}

func (s *Store) saveGroups(list []proto.NoteGroup) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	tmp := s.groupsFile() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	return os.Rename(tmp, s.groupsFile())
}

// ensureGroup makes group name if there is none; s.mu is held.
func (s *Store) ensureGroup(name string) error {
	list, err := s.Groups()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(list, func(g proto.NoteGroup) bool { return g.Name == name }) {
		return nil
	}
	return s.saveGroups(append(list, proto.NoteGroup{Name: name}))
}

// SaveGroup makes group g, or changes group from into g: its notes follow
// a new name.
func (s *Store) SaveGroup(from string, g proto.NoteGroup) error {
	g.Name = strings.Join(strings.Fields(g.Name), " ")
	if g.Name == "" {
		return errors.New("notes: a group needs a name")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.Groups()
	if err != nil {
		return err
	}
	if g.Name != from && slices.ContainsFunc(list, func(o proto.NoteGroup) bool { return o.Name == g.Name }) {
		return fmt.Errorf("notes: there is a group %q already", g.Name)
	}
	icon, err := checkIcon(g.Icon)
	if err != nil {
		return err
	}
	g.Icon, g.IconReview = icon, false
	if from == "" {
		if err := s.saveGroups(append(list, g)); err != nil {
			return err
		}
		return s.usedIcon(icon)
	}
	i := slices.IndexFunc(list, func(o proto.NoteGroup) bool { return o.Name == from })
	if i < 0 {
		return fmt.Errorf("notes: no group %q", from)
	}
	if list[i].Icon == g.Icon {
		g.IconReview, icon = list[i].IconReview, ""
	}
	list[i] = g
	if err := s.saveGroups(list); err != nil {
		return err
	}
	if err := s.usedIcon(icon); err != nil {
		return err
	}
	if g.Name != from {
		return s.regroup(from, g.Name)
	}
	return nil
}

// DeleteGroup forgets group name, its notes left in none.
func (s *Store) DeleteGroup(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.Groups()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(o proto.NoteGroup) bool { return o.Name == name })
	if i < 0 {
		return fmt.Errorf("notes: no group %q", name)
	}
	if err := s.regroup(name, ""); err != nil {
		return err
	}
	return s.saveGroups(slices.Delete(list, i, i+1))
}

// MoveGroup puts group name before group before, or last.
func (s *Store) MoveGroup(name, before string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.Groups()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(list, func(o proto.NoteGroup) bool { return o.Name == name })
	if i < 0 {
		return fmt.Errorf("notes: no group %q", name)
	}
	g := list[i]
	list = slices.Delete(list, i, i+1)
	at := len(list)
	if before != "" {
		if at = slices.IndexFunc(list, func(o proto.NoteGroup) bool { return o.Name == before }); at < 0 {
			return fmt.Errorf("notes: no group %q", before)
		}
	}
	return s.saveGroups(slices.Insert(list, at, g))
}

// SetGroup puts note id in group ("" for none), at the top level, last.
func (s *Store) SetGroup(id, group string) (proto.NoteInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if group != "" {
		if err := s.ensureGroup(group); err != nil {
			return proto.NoteInfo{}, err
		}
	}
	n, err := s.read(id, true)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	list, err := s.List()
	if err != nil {
		return proto.NoteInfo{}, err
	}
	if n.Parent != "" {
		n.Order = 0
		for _, o := range list {
			if o.Parent == "" && o.Order >= n.Order {
				n.Order = o.Order + 1
			}
		}
	}
	n.Parent, n.Group = "", group
	return n.NoteInfo, s.write(n)
}

// regroup moves the notes of group from to group to; s.mu is held.
func (s *Store) regroup(from, to string) error {
	list, err := s.List()
	if err != nil {
		return err
	}
	for _, info := range list {
		if info.Group != from || info.Parent != "" {
			continue
		}
		n, err := s.read(info.ID, true)
		if err != nil {
			return err
		}
		n.Group = to
		if err := s.write(n); err != nil {
			return err
		}
	}
	return nil
}
