package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// Claude Code records each session as JSON lines in
// ~/.claude/projects/<folder>/<session>.jsonl: user and assistant records
// whose message content is text or blocks (text, thinking, tool_use,
// tool_result). Records marked isMeta or isSidechain (subagents) aren't
// the conversation. See docs/chat-view.md.

// ClaudeFiles returns the transcripts Claude Code keeps for sessions run in
// dir, newest first.
func ClaudeFiles(home, dir string) []string {
	root := filepath.Join(home, ".claude", "projects")
	var folders []string
	if f := filepath.Join(root, claudeFolder(dir)); isDir(f) {
		folders = append(folders, f)
	} else {
		// The folder name is derived from the path; when it isn't where
		// expected, find the folder whose records ran in dir.
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			f := filepath.Join(root, e.Name())
			if e.IsDir() && claudeFolderRunsIn(f, dir) {
				folders = append(folders, f)
			}
		}
	}
	type file struct {
		path string
		mod  time.Time
	}
	var files []file
	for _, folder := range folders {
		entries, _ := os.ReadDir(folder)
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
				continue
			}
			if info, err := e.Info(); err == nil {
				files = append(files, file{filepath.Join(folder, e.Name()), info.ModTime()})
			}
		}
	}
	out := make([]string, 0, len(files))
	for len(files) > 0 {
		newest := 0
		for i, f := range files {
			if f.mod.After(files[newest].mod) {
				newest = i
			}
		}
		out = append(out, files[newest].path)
		files = append(files[:newest], files[newest+1:]...)
	}
	return out
}

var notNameChar = regexp.MustCompile(`[^A-Za-z0-9]`)

// claudeFolder is how Claude Code names the folder of a working directory.
func claudeFolder(dir string) string { return notNameChar.ReplaceAllString(dir, "-") }

func claudeFolderRunsIn(folder, dir string) bool {
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		f, err := os.Open(filepath.Join(folder, e.Name()))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 4<<20)
		for i := 0; i < 20 && sc.Scan(); i++ {
			var rec struct {
				Cwd string `json:"cwd"`
			}
			if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Cwd != "" {
				f.Close()
				return rec.Cwd == dir
			}
		}
		f.Close()
	}
	return false
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// ModTime is when the file last changed, zero when it is missing.
func ModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// NewClaudeReader follows a Claude Code transcript.
func NewClaudeReader(path string) *Reader { return newReader(path, claudeParser{}) }

type claudeParser struct{}

type claudeRecord struct {
	Type        string    `json:"type"`
	IsMeta      bool      `json:"isMeta"`
	IsSidechain bool      `json:"isSidechain"`
	Timestamp   time.Time `json:"timestamp"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

func (claudeParser) parse(line []byte, r *Reader) {
	var rec claudeRecord
	if json.Unmarshal(line, &rec) != nil || rec.IsMeta || rec.IsSidechain {
		return
	}
	if rec.Type != "user" && rec.Type != "assistant" {
		return
	}
	var text string
	if json.Unmarshal(rec.Message.Content, &text) == nil {
		addClaudeText(r, rec, text)
		return
	}
	var blocks []claudeBlock
	if json.Unmarshal(rec.Message.Content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			addClaudeText(r, rec, b.Text)
		case "thinking":
			if strings.TrimSpace(b.Thinking) != "" {
				r.add(proto.TranscriptEntry{Kind: proto.TranscriptThinking, Time: rec.Timestamp, Text: b.Thinking})
			}
		case "tool_use":
			i := r.add(proto.TranscriptEntry{Kind: proto.TranscriptTool, Time: rec.Timestamp, Text: b.Name, Detail: toolDetail(b.Input)})
			r.tools[b.ID] = i
		case "tool_result":
			i, ok := r.tools[b.ToolUseID]
			if !ok {
				continue
			}
			result := clip(strings.TrimSpace(blockText(b.Content)), 4000)
			r.change(i, func(e *proto.TranscriptEntry) {
				e.Done, e.Failed, e.Result = true, b.IsError, result
			})
		}
	}
}

var commandName = regexp.MustCompile(`<command-name>([^<]*)</command-name>`)

func addClaudeText(r *Reader, rec claudeRecord, text string) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return
	case strings.HasPrefix(text, "<command-name>") || strings.HasPrefix(text, "<command-message>"):
		if m := commandName.FindStringSubmatch(text); m != nil {
			r.add(proto.TranscriptEntry{Kind: proto.TranscriptNotice, Time: rec.Timestamp, Text: "Ran " + m[1]})
		}
		return
	case strings.HasPrefix(text, "<local-command-stdout>") || strings.HasPrefix(text, "<local-command-stderr>"),
		strings.HasPrefix(text, "Caveat: The messages below were generated by the user while running local commands"):
		return
	case strings.HasPrefix(text, "[Request interrupted"):
		r.add(proto.TranscriptEntry{Kind: proto.TranscriptNotice, Time: rec.Timestamp, Text: "Interrupted"})
		return
	}
	kind := proto.TranscriptUser
	if rec.Type == "assistant" {
		kind = proto.TranscriptAssistant
	}
	r.add(proto.TranscriptEntry{Kind: kind, Time: rec.Timestamp, Text: text})
}

// toolDetail sums up what a tool call does from its input.
func toolDetail(input json.RawMessage) string {
	var in map[string]any
	if json.Unmarshal(input, &in) != nil {
		return ""
	}
	for _, key := range []string{"command", "file_path", "path", "pattern", "url", "query", "description", "prompt"} {
		if v, ok := in[key].(string); ok && strings.TrimSpace(v) != "" {
			return clip(strings.TrimSpace(v), 300)
		}
	}
	raw, _ := json.Marshal(in)
	return clip(string(raw), 300)
}

// blockText is the text of a tool result: a string or text blocks.
func blockText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}
