package daemon

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/transcript"
	"github.com/enowx/enowx/proto"
)

// transcripts follows the transcript of each session shown as a chat.
// The map and each follower's used are guarded by mu; the rest of a
// follower by its own lock, held while it reads its file.
type transcripts struct {
	mu    sync.Mutex
	byID  map[string]*follower
	files map[string]string // transcript file → the session following it
}

type follower struct {
	mu      sync.Mutex
	reader  *transcript.Reader
	checked time.Time // when we last looked for a newer file
	used    time.Time // when the app last asked
}

// dropAfter is how long a conversation no chat shows stays in memory; a
// chat shown polls every second or so, and one shown again reads it anew.
const dropAfter = 5 * time.Minute

// lookEvery is how often a session's folder is searched for a newer
// transcript: an agent starts a new file on /clear, for example.
const lookEvery = 2 * time.Second

func (t *transcripts) follower(id string) *follower {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byID == nil {
		t.byID, t.files = map[string]*follower{}, map[string]string{}
	}
	now := time.Now()
	for other, g := range t.byID {
		if now.Sub(g.used) > dropAfter {
			delete(t.byID, other) // its file stays claimed
		}
	}
	f := t.byID[id]
	if f == nil {
		f = &follower{}
		t.byID[id] = f
	}
	f.used = now
	return f
}

// claim gives path to session id unless another session follows it.
func (t *transcripts) claim(id, path, old string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if owner, ok := t.files[path]; ok && owner != id {
		return false
	}
	delete(t.files, old)
	t.files[path] = id
	return true
}

func (t *transcripts) forget(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byID, id)
	for path, owner := range t.files {
		if owner == id {
			delete(t.files, path)
		}
	}
}

// transcript returns what changed in session id's conversation.
func (s *Server) transcript(p proto.TranscriptParams) (proto.Transcript, error) {
	sess, err := s.sessions.Get(p.ID)
	if err != nil {
		return proto.Transcript{}, err
	}
	info := sess.Info()
	name := "This session"
	if preset, ok := agent.Find(info.Agent); ok {
		name = preset.Name
	}
	if !transcript.Supports(info.Agent) {
		return proto.Transcript{Reason: name + " has no chat view yet; it shows in the terminal."}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return proto.Transcript{}, err
	}
	f := s.transcripts.follower(p.ID)
	f.mu.Lock()
	defer f.mu.Unlock()
	reset := false
	if f.reader == nil || time.Since(f.checked) > lookEvery {
		f.checked = time.Now()
		for _, found := range transcript.Find(info.Agent, home, info.Dir) {
			// Conversations older than the session belong to earlier ones;
			// a restored session's changes once its agent continues.
			if found.Changed.Before(info.Created.Add(-time.Second)) {
				break
			}
			if f.reader != nil && f.reader.Path() == found.Path {
				break
			}
			old := ""
			if f.reader != nil {
				old = f.reader.Path()
			}
			if s.transcripts.claim(p.ID, found.Path, old) {
				f.reader, reset = transcript.Open(info.Agent, found.Path), true
				break
			}
		}
	}
	if f.reader == nil {
		return proto.Transcript{Supported: true, Reason: "Nothing yet: the conversation shows here once " + name + " records it."}, nil
	}
	if err := f.reader.Update(); err != nil {
		return proto.Transcript{}, err
	}
	since := p.Since
	if reset {
		since = 0
	}
	var entries []proto.TranscriptEntry
	var rev int64
	switch {
	case p.Before > 0 && !reset:
		entries, rev = f.reader.Before(p.Before, max(p.Tail, 1))
	case since == 0 && p.Tail > 0:
		entries, rev = f.reader.Before(f.reader.Len(), p.Tail)
	default:
		entries, rev = f.reader.Since(since)
	}
	out := proto.Transcript{Supported: true, Entries: entries, Rev: rev, Reset: reset,
		Model: f.reader.Model, Mode: f.reader.Mode, Effort: f.reader.Effort,
		ContextTokens: f.reader.Context}
	if out.ContextTokens > 0 {
		out.ContextWindow = f.reader.Window
		if out.ContextWindow == 0 {
			out.ContextWindow = agent.SettingsOf(info.Agent).ContextWindow(f.reader.Model)
		}
	}
	// The screen says a mode change at once; the transcript only with
	// the next message.
	if m := agent.SettingsOf(info.Agent).ModeShown(sess.Terminal().Text()); m != "" {
		out.Mode = m
	}
	return out, nil
}

// switchMode cycles session id's agent to mode, watching its screen, on a
// goroutine of its own: requests on a connection run in order, and each
// step waits for the agent to redraw.
func (s *Server) switchMode(p proto.SessionModeParams) error {
	sess, err := s.sessions.Get(p.ID)
	if err != nil {
		return err
	}
	set := agent.SettingsOf(sess.Info().Agent)
	if set.ModeKey == "" {
		return errors.New("this agent's mode can't be changed while it runs")
	}
	if _, ok := agent.FindChoice(set.Modes, p.Mode); !ok {
		return fmt.Errorf("unknown mode %q", p.Mode)
	}
	go func() {
		for range len(set.Modes) + 1 {
			if set.ModeShown(sess.Terminal().Text()) == p.Mode {
				return
			}
			s.sessions.Input(sess, []byte(set.ModeKey))
			time.Sleep(modeStep)
		}
	}()
	return nil
}

// modeStep is how long the agent gets to redraw its mode after a key.
const modeStep = 350 * time.Millisecond

// transcriptPath is the file of session id's agent's conversation, as the
// chat finds it; "" when there is none it knows.
func (s *Server) transcriptPath(id string) string {
	if _, err := s.transcript(proto.TranscriptParams{ID: id, Tail: 1}); err != nil {
		return ""
	}
	f := s.transcripts.follower(id)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reader == nil {
		return ""
	}
	return f.reader.Path()
}
