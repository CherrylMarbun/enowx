package transcript

import "time"

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
}{
	"claude":       {claudeFound, NewClaudeReader},
	"claude-teams": {claudeFound, NewClaudeReader},
	"codex":        {CodexFiles, NewCodexReader},
}

// Supports reports whether the conversations of agent (a preset ID) can
// be read.
func Supports(agent string) bool {
	_, ok := readers[agent]
	return ok
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
