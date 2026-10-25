package canvas

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// A canvas's conversation with the Canvas page's agent is kept with it
// (chat.json), so every host shows it and which agent it talks to.

func (s *Store) chatFile(id string) string { return filepath.Join(s.dir, id, "chat.json") }

// ChatLog is canvas id's conversation.
func (s *Store) ChatLog(id string) (proto.CanvasChatLog, error) {
	if _, err := s.Get(id); err != nil {
		return proto.CanvasChatLog{}, err
	}
	log := proto.CanvasChatLog{ID: id, Messages: []proto.CanvasChatEntry{}}
	if err := readJSON(s.chatFile(id), &log); err != nil && !os.IsNotExist(err) {
		return proto.CanvasChatLog{}, fmt.Errorf("canvas: chat: %w", err)
	}
	log.ID = id
	if log.Mode == "" {
		log.Mode = proto.CanvasChatReview
	}
	return log, nil
}

// Chat records p: a message sent, with the canvas's version now, or the
// agent or mode picked.
func (s *Store) Chat(p proto.CanvasChatParams) (proto.CanvasChatLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log, err := s.ChatLog(p.ID)
	if err != nil {
		return log, err
	}
	if p.Agent != "" {
		log.Agent = p.Agent
	}
	switch p.Mode {
	case "":
	case proto.CanvasChatReview, proto.CanvasChatDirect:
		log.Mode = p.Mode
	default:
		return log, fmt.Errorf("canvas: no chat mode %q: review or direct", p.Mode)
	}
	if text := strings.TrimSpace(p.Text); text != "" {
		c, err := s.Get(p.ID)
		if err != nil {
			return log, err
		}
		n := 1
		if k := len(log.Messages); k > 0 {
			n = log.Messages[k-1].N + 1
		}
		log.Messages = append(log.Messages, proto.CanvasChatEntry{N: n, Time: time.Now().UTC(), Text: text, Files: p.Files,
			Version: c.Version, Session: p.Session})
	}
	return log, writeJSON(s.chatFile(p.ID), log)
}

// ChatMark says what became of message n's changes.
func (s *Store) ChatMark(m proto.CanvasChatMark) (proto.CanvasChatLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	log, err := s.ChatLog(m.ID)
	if err != nil {
		return log, err
	}
	if m.State != proto.CanvasChatKept && m.State != proto.CanvasChatUndone {
		return log, errors.New("canvas: a message's changes are kept or undone")
	}
	i := slices.IndexFunc(log.Messages, func(e proto.CanvasChatEntry) bool { return e.N == m.N })
	if i < 0 {
		return log, fmt.Errorf("canvas: no message %d", m.N)
	}
	log.Messages[i].State = m.State
	return log, writeJSON(s.chatFile(m.ID), log)
}

// ChatContext is the line a message to a canvas's agent starts with:
// the canvas, the design shown (a number, name or ID), the element
// selected.
func ChatContext(c proto.Canvas, design, element string) string {
	parts := []string{fmt.Sprintf("canvas %s “%s”", c.ID, c.Title)}
	if i, err := FrameIndex(c, design); err == nil && len(c.Frames) > 0 {
		f := c.Frames[i]
		parts = append(parts, fmt.Sprintf("design %d “%s” (frame %s, %d×%d)", i+1, f.Name, f.ID, f.Width, f.Height))
	}
	if element != "" {
		parts = append(parts, "selected #"+element)
	}
	return "[" + strings.Join(parts, " · ") + "]\n"
}

// WithoutContext is a message to a canvas's agent without the line
// ChatContext started it with, as the user wrote it.
func WithoutContext(text string) string {
	if !strings.HasPrefix(text, "[canvas ") {
		return text
	}
	if _, rest, ok := strings.Cut(text, "]\n"); ok {
		return rest
	}
	return text
}
