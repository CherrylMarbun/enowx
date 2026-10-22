package daemon

import (
	"os"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/transcript"
	"github.com/enowx/enowx/proto"
)

// transcripts follows the transcript of each session shown as a chat.
// The map is guarded by mu; each follower by its own lock, held while it
// reads its file.
type transcripts struct {
	mu    sync.Mutex
	byID  map[string]*follower
	files map[string]string // transcript file → the session following it
}

type follower struct {
	mu      sync.Mutex
	reader  *transcript.Reader
	checked time.Time // when we last looked for a newer file
}

// lookEvery is how often a session's folder is searched for a newer
// transcript: an agent starts a new file on /clear, for example.
const lookEvery = 2 * time.Second

func (t *transcripts) follower(id string) *follower {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byID == nil {
		t.byID, t.files = map[string]*follower{}, map[string]string{}
	}
	f := t.byID[id]
	if f == nil {
		f = &follower{}
		t.byID[id] = f
	}
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
	if info.Agent != "claude" {
		name := "This session"
		if preset, ok := agent.Find(info.Agent); ok {
			name = preset.Name
		}
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
		for _, path := range transcript.ClaudeFiles(home, info.Dir) {
			// Files older than the session belong to earlier ones; a
			// restored session's file changes once its agent continues.
			if transcript.ModTime(path).Before(info.Created.Add(-time.Second)) {
				break
			}
			if f.reader != nil && f.reader.Path() == path {
				break
			}
			old := ""
			if f.reader != nil {
				old = f.reader.Path()
			}
			if s.transcripts.claim(p.ID, path, old) {
				f.reader, reset = transcript.NewClaudeReader(path), true
				break
			}
		}
	}
	if f.reader == nil {
		return proto.Transcript{Supported: true, Reason: "Nothing yet: the conversation shows here once Claude Code records it."}, nil
	}
	if err := f.reader.Update(); err != nil {
		return proto.Transcript{}, err
	}
	since := p.Since
	if reset {
		since = 0
	}
	entries, rev := f.reader.Since(since)
	return proto.Transcript{Supported: true, Entries: entries, Rev: rev, Reset: reset}, nil
}
