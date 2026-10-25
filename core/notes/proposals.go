package notes

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// Proposals: changes agents propose to notes, kept until the user accepts
// or rejects them, in proposals.json beside the notes.

func (s *Store) proposalsFile() string { return filepath.Join(s.dir, "proposals.json") }

// Proposals is the changes waiting for the user, oldest first.
func (s *Store) Proposals() ([]proto.NoteProposal, error) {
	data, err := os.ReadFile(s.proposalsFile())
	if errors.Is(err, os.ErrNotExist) {
		return []proto.NoteProposal{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notes: %w", err)
	}
	out := []proto.NoteProposal{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("notes: proposals: %w", err)
	}
	return out, nil
}

func (s *Store) saveProposals(list []proto.NoteProposal) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	tmp := s.proposalsFile() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	return os.Rename(tmp, s.proposalsFile())
}

// Propose keeps n as a change to note n.ID (a new note when ""), made by
// session, replacing what that session proposed for it before.
func (s *Store) Propose(n proto.Note, session string) (proto.NoteProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := proto.NoteProposal{Note: n, Session: session, Created: time.Now().UTC().Truncate(time.Second)}
	if n.ID != "" {
		cur, err := s.read(n.ID, true)
		if err != nil {
			return proto.NoteProposal{}, err
		}
		p.Before = cur.Body
		if strings.TrimSpace(n.Title) == "" {
			p.Note.Title = cur.Title
		}
		p.Note.Parent, p.Note.Icon = cur.Parent, orString(n.Icon, cur.Icon)
	} else if n.Parent != "" {
		if _, err := s.read(n.Parent, false); err != nil {
			return proto.NoteProposal{}, err
		}
	}
	list, err := s.Proposals()
	if err != nil {
		return proto.NoteProposal{}, err
	}
	if n.ID != "" {
		list = slices.DeleteFunc(list, func(o proto.NoteProposal) bool { return o.Note.ID == n.ID && o.Session == session })
	}
	var b [6]byte
	rand.Read(b[:])
	p.ID = hex.EncodeToString(b[:])
	list = append(list, p)
	return p, s.saveProposals(list)
}

func orString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// take removes proposal id and returns it; s.mu is held.
func (s *Store) take(id string) (proto.NoteProposal, error) {
	list, err := s.Proposals()
	if err != nil {
		return proto.NoteProposal{}, err
	}
	i := slices.IndexFunc(list, func(p proto.NoteProposal) bool { return p.ID == id })
	if i < 0 {
		return proto.NoteProposal{}, fmt.Errorf("notes: no proposal %q", id)
	}
	p := list[i]
	return p, s.saveProposals(slices.Delete(list, i, i+1))
}

// Accept makes proposal id's change and forgets it.
func (s *Store) Accept(id string) (proto.NoteInfo, error) {
	s.mu.Lock()
	p, err := s.take(id)
	if err == nil && p.Note.ID != "" {
		s.keep(p.Note.ID, true) // what the agent changed can be undone later too
	}
	s.mu.Unlock()
	if err != nil {
		return proto.NoteInfo{}, err
	}
	return s.Save(p.Note)
}

// Reject forgets proposal id.
func (s *Store) Reject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.take(id)
	return err
}
