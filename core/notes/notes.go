// Package notes keeps a host's notes: pages nested like a notebook's,
// each a Markdown file of its own with a header of what it is, so the
// user, their agents and any editor can read them.
package notes

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Store is the notes in a folder, one <id>.md each.
type Store struct {
	dir string
	mu  sync.Mutex // one change at a time
}

// Open is the store in dir, made when the first note is saved.
func Open(dir string) *Store { return &Store{dir: dir} }

var idPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

func (s *Store) file(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("notes: no note %q", id)
	}
	return filepath.Join(s.dir, id+".md"), nil
}

// List is every note without its body, in the tree's order: by parent,
// then Order.
func (s *Store) List() ([]proto.NoteInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []proto.NoteInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notes: %w", err)
	}
	out := []proto.NoteInfo{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || !idPattern.MatchString(id) {
			continue
		}
		n, err := s.read(id, false)
		if err != nil {
			continue // half written, or not a note
		}
		out = append(out, n.NoteInfo)
	}
	sortNotes(out)
	return out, nil
}

func sortNotes(list []proto.NoteInfo) {
	slices.SortStableFunc(list, func(a, b proto.NoteInfo) int {
		if a.Parent != b.Parent {
			return strings.Compare(a.Parent, b.Parent)
		}
		if a.Order != b.Order {
			return a.Order - b.Order
		}
		return a.Created.Compare(b.Created)
	})
}

// Get is note id with its body.
func (s *Store) Get(id string) (proto.Note, error) { return s.read(id, true) }

// Save writes n: a new note when it has no ID, under n.Parent, last; else
// note n.ID's title, icon and body (its place moves with Move).
func (s *Store) Save(n proto.Note) (proto.NoteInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n.Title = strings.Join(strings.Fields(n.Title), " ")
	n.Icon = strings.TrimSpace(strings.ReplaceAll(n.Icon, "\n", ""))
	now := time.Now().UTC().Truncate(time.Second)
	if n.ID == "" {
		if n.Parent != "" {
			if _, err := s.read(n.Parent, false); err != nil {
				return proto.NoteInfo{}, err
			}
		}
		var b [6]byte
		rand.Read(b[:])
		n.ID, n.Created = hex.EncodeToString(b[:]), now
		list, err := s.List()
		if err != nil {
			return proto.NoteInfo{}, err
		}
		n.Order = 0
		for _, o := range list {
			if o.Parent == n.Parent && o.Order >= n.Order {
				n.Order = o.Order + 1
			}
		}
	} else {
		old, err := s.read(n.ID, false)
		if err != nil {
			return proto.NoteInfo{}, err
		}
		n.Parent, n.Order, n.Created = old.Parent, old.Order, old.Created
	}
	n.Updated = now
	if err := s.write(n); err != nil {
		return proto.NoteInfo{}, err
	}
	return n.NoteInfo, nil
}

// Delete removes note id and the notes under it.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(id, false); err != nil {
		return err
	}
	list, err := s.List()
	if err != nil {
		return err
	}
	gone := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for _, n := range list {
			if !gone[n.ID] && gone[n.Parent] {
				gone[n.ID], changed = true, true
			}
		}
	}
	for id := range gone {
		path, _ := s.file(id)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("notes: %w", err)
		}
	}
	// What agents proposed for them goes with them.
	props, err := s.Proposals()
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(props, func(p proto.NoteProposal) bool { return gone[p.Note.ID] || gone[p.Note.Parent] })
	if len(kept) == len(props) {
		return nil
	}
	return s.saveProposals(kept)
}

// Move puts note id under parent ("" for the top), before its child
// before, or last with before "".
func (s *Store) Move(id, parent, before string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.List()
	if err != nil {
		return err
	}
	byID := map[string]proto.NoteInfo{}
	for _, n := range list {
		byID[n.ID] = n
	}
	if _, ok := byID[id]; !ok {
		return fmt.Errorf("notes: no note %q", id)
	}
	for p := parent; p != ""; p = byID[p].Parent {
		if p == id {
			return errors.New("notes: a note can't go under itself")
		}
		if _, ok := byID[p]; !ok {
			return fmt.Errorf("notes: no note %q", p)
		}
	}
	var siblings []string
	for _, n := range list {
		if n.Parent == parent && n.ID != id {
			siblings = append(siblings, n.ID)
		}
	}
	at := len(siblings)
	if before != "" {
		if at = slices.Index(siblings, before); at < 0 {
			return fmt.Errorf("notes: %q is not under %q", before, parent)
		}
	}
	siblings = slices.Insert(siblings, at, id)
	for i, sid := range siblings {
		n := byID[sid]
		if n.Order == i && n.Parent == parent {
			continue
		}
		full, err := s.read(sid, true)
		if err != nil {
			return err
		}
		full.Order, full.Parent = i, parent
		if err := s.write(full); err != nil {
			return err
		}
	}
	return nil
}

// A note's file: a header between "---" lines, "key: value" each, then
// the body.

func (s *Store) read(id string, body bool) (proto.Note, error) {
	path, err := s.file(id)
	if err != nil {
		return proto.Note{}, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return proto.Note{}, fmt.Errorf("notes: no note %q", id)
	}
	if err != nil {
		return proto.Note{}, fmt.Errorf("notes: %w", err)
	}
	defer f.Close()
	n := proto.Note{NoteInfo: proto.NoteInfo{ID: id}}
	r := bufio.NewReader(f)
	first, _ := r.ReadString('\n')
	if strings.TrimSpace(first) != "---" {
		return proto.Note{}, fmt.Errorf("notes: %s has no header", path)
	}
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "---" {
			break
		}
		if err != nil {
			return proto.Note{}, fmt.Errorf("notes: %s's header doesn't end", path)
		}
		key, value, _ := strings.Cut(line, ":")
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "title":
			n.Title = value
		case "icon":
			n.Icon = value
		case "parent":
			n.Parent = value
		case "order":
			n.Order, _ = strconv.Atoi(value)
		case "created":
			n.Created, _ = time.Parse(time.RFC3339, value)
		case "updated":
			n.Updated, _ = time.Parse(time.RFC3339, value)
		}
	}
	if body {
		var b strings.Builder
		_, err := r.WriteTo(&b)
		if err != nil {
			return proto.Note{}, fmt.Errorf("notes: %w", err)
		}
		n.Body = strings.TrimPrefix(b.String(), "\n")
	}
	return n, nil
}

func (s *Store) write(n proto.Note) error {
	path, err := s.file(n.ID)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", n.Title)
	if n.Icon != "" {
		fmt.Fprintf(&b, "icon: %s\n", n.Icon)
	}
	if n.Parent != "" {
		fmt.Fprintf(&b, "parent: %s\n", n.Parent)
	}
	fmt.Fprintf(&b, "order: %d\n", n.Order)
	fmt.Fprintf(&b, "created: %s\n", n.Created.Format(time.RFC3339))
	fmt.Fprintf(&b, "updated: %s\n", n.Updated.Format(time.RFC3339))
	b.WriteString("---\n\n")
	b.WriteString(n.Body)
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	return os.Rename(tmp, path)
}
