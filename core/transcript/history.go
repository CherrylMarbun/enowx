package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// History lists the conversations agents had in dirs (a project's folder
// and its worktrees), newest first, at most limit: for picking one to go
// on with. Agents whose conversations this package reads have one.
func History(home string, dirs []string, limit int) []proto.Conversation {
	var out []proto.Conversation
	for _, agentID := range []string{"claude", "codex"} {
		for _, dir := range dirs {
			for _, f := range Find(agentID, home, dir) {
				c, ok := conversation(agentID, f, dir)
				if ok {
					out = append(out, c)
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b proto.Conversation) int { return b.Updated.Compare(a.Updated) })
	// A conversation in two of dirs (a worktree inside the project's
	// folder, say) once.
	out = slices.CompactFunc(out, func(a, b proto.Conversation) bool { return a.Agent == b.Agent && a.ID == b.ID })
	return out[:min(len(out), limit)]
}

// conversation reads what History says of the conversation in f.
func conversation(agentID string, f Found, dir string) (proto.Conversation, bool) {
	c := proto.Conversation{Agent: agentID, Dir: dir, Updated: f.Changed, Path: f.Path}
	switch agentID {
	case "claude":
		c.ID = strings.TrimSuffix(filepath.Base(f.Path), ".jsonl")
	case "codex":
		c.ID = codexSessionID(f.Path)
	}
	if c.ID == "" {
		return c, false
	}
	r := Open(agentID, f.Path)
	if r == nil || r.Update() != nil {
		return c, false
	}
	entries, _ := r.Since(0)
	for _, e := range entries {
		switch e.Kind {
		case proto.TranscriptUser:
			if c.Title == "" {
				c.Title = firstLine(e.Text)
			}
			c.Messages++
		case proto.TranscriptAssistant:
			c.Messages++
		}
	}
	c.Model = r.Model
	return c, c.Messages > 0
}

// codexSessionID is the ID a Codex rollout's session goes by, which
// `codex resume` takes: its session_meta's.
func codexSessionID(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	if !sc.Scan() {
		return ""
	}
	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			ID string `json:"id"`
		} `json:"payload"`
	}
	if json.Unmarshal(sc.Bytes(), &meta) != nil || meta.Type != "session_meta" {
		return ""
	}
	return meta.Payload.ID
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(line); len(r) > 120 {
		line = string(r[:120]) + "…"
	}
	return line
}
