package transcript

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Found is a conversation an agent recorded, by the path a Reader opens it
// with, and when it last changed.
type Found struct {
	Path    string
	Changed time.Time
}

// readers are the agents whose conversations this package reads.
var readers = map[string]struct {
	find func(home, dir string) []Found
	open func(path string) *Reader
	id   func(path string) string // the id the agent resumes it by; nil: none
}{
	"claude":             {claudeFound, NewClaudeReader, claudeID},
	"claude-agent-teams": {claudeFound, NewClaudeReader, claudeID},
	"codex":              {CodexFiles, NewCodexReader, nil},
	"omp":                {OMPFiles, NewOMPReader, OMPConversationID},
}

func claudeID(path string) string { return strings.TrimSuffix(filepath.Base(path), ".jsonl") }

// ConversationID is the id agent resumes the conversation at path by, ""
// when it has none to give.
func ConversationID(agent, path string) string {
	if r, ok := readers[agent]; ok && r.id != nil && path != "" {
		return r.id(path)
	}
	return ""
}

// Supports reports whether the conversations of agent (a preset ID) can
// be read.
func Supports(agent string) bool {
	_, ok := readers[agent]
	return ok
}

// FindIn lists the conversations agent had in dir, newest change first:
// those in store when the agent kept them there (a session's own folder,
// Preset.SessionDir), else Find's.
func FindIn(agent, home, dir, store string) []Found {
	if store != "" {
		var out []Found
		entries, _ := os.ReadDir(store)
		for _, e := range entries {
			if info, err := e.Info(); err == nil && !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") && !strings.HasPrefix(e.Name(), ".") {
				out = append(out, Found{Path: filepath.Join(store, e.Name()), Changed: info.ModTime()})
			}
		}
		if len(out) > 0 {
			slices.SortFunc(out, func(a, b Found) int { return b.Changed.Compare(a.Changed) })
			return out
		}
	}
	return Find(agent, home, dir)
}

// Find lists the conversations agent had in dir, newest change first.
func Find(agent, home, dir string) []Found {
	if r, ok := readers[agent]; ok {
		return r.find(home, dir)
	}
	return nil
}

// Open follows a conversation Find listed for agent; nil for an agent it
// doesn't read.
func Open(agent, path string) *Reader {
	if r, ok := readers[agent]; ok {
		return r.open(path)
	}
	return nil
}

func claudeFound(home, dir string) []Found {
	files := ClaudeFiles(home, dir)
	out := make([]Found, len(files))
	for i, f := range files {
		out[i] = Found{Path: f, Changed: ModTime(f)}
	}
	return out
}
