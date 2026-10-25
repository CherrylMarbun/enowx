package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// A note's conversations with its agent in the Notes page, kept in
// chats/<id>.json beside the notes so they are the same on every host:
// what the user asked and what the agent answered, turn by turn.

func (s *Store) chatFile(id string) string { return filepath.Join(s.dir, "chats", id+".json") }

// SetAgent picks the agent note id's page talks to.
func (s *Store) SetAgent(id, agent string) (proto.NoteInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.read(id, true)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	n.Agent = strings.TrimSpace(agent)
	if err := s.write(n); err != nil {
		return proto.NoteInfo{}, err
	}
	return n.NoteInfo, nil
}

// ChatLog is note id's conversations, as kept.
func (s *Store) ChatLog(id string) (proto.NoteChatLog, error) {
	if _, err := s.file(id); err != nil {
		return proto.NoteChatLog{}, err
	}
	log := proto.NoteChatLog{Note: id, Messages: []proto.NoteChatMessage{}}
	data, err := os.ReadFile(s.chatFile(id))
	if errors.Is(err, fs.ErrNotExist) {
		return log, nil
	}
	if err != nil {
		return log, fmt.Errorf("notes: %w", err)
	}
	if err := json.Unmarshal(data, &log); err != nil {
		return log, fmt.Errorf("notes: chat %s: %w", id, err)
	}
	return log, nil
}

// KeepChat adds a session's messages to note id's conversations: those of
// session already kept are replaced by msgs, the others stay.
func (s *Store) KeepChat(id, session string, msgs []proto.NoteChatMessage) (proto.NoteChatLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log, err := s.ChatLog(id)
	if err != nil {
		return log, err
	}
	kept := slices.DeleteFunc(log.Messages, func(m proto.NoteChatMessage) bool { return m.Session == session })
	before := len(kept)
	kept = append(kept, msgs...)
	slices.SortStableFunc(kept, func(a, b proto.NoteChatMessage) int { return a.Time.Compare(b.Time) })
	if len(kept) == before+len(msgs) && slices.EqualFunc(kept, log.Messages, func(a, b proto.NoteChatMessage) bool { return a == b }) {
		return log, nil // nothing new
	}
	log.Messages, log.Updated = kept, time.Now().UTC()
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return log, err
	}
	if err := os.MkdirAll(filepath.Dir(s.chatFile(id)), 0o700); err != nil {
		return log, fmt.Errorf("notes: %w", err)
	}
	tmp := s.chatFile(id) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return log, fmt.Errorf("notes: %w", err)
	}
	return log, os.Rename(tmp, s.chatFile(id))
}
