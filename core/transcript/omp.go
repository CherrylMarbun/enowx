package transcript

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

var ompUUID = regexp.MustCompile(`_([0-9a-fA-F-]+)\.jsonl$`)

// OMPSessionFiles returns transcripts for a session directly from its dedicated session directory if it exists,
// or falls back to global sessions.
func OMPSessionFiles(home, dir, sessionID string) []Found {
	var all []Found
	if sessionID != "" {
		sessDir := filepath.Join(home, ".enowx-app", "omp-sessions", sessionID)
		if files, err := os.ReadDir(sessDir); err == nil {
			for _, f := range files {
				if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") || strings.HasPrefix(f.Name(), ".") {
					continue
				}
				path := filepath.Join(sessDir, f.Name())
				if info, err := f.Info(); err == nil {
					all = append(all, Found{Path: path, Changed: info.ModTime()})
				}
			}
		}
	}
	if len(all) > 0 {
		slices.SortFunc(all, func(a, b Found) int {
			return b.Changed.Compare(a.Changed)
		})
		return all
	}
	return OMPFiles(home, dir)
}

// OMPFiles returns the transcripts OMP keeps for sessions run in dir, newest first.
func OMPFiles(home, dir string) []Found {
	cleanDir := filepath.Clean(dir)
	var all []Found
	root := filepath.Join(home, ".omp", "agent", "sessions")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		folder := filepath.Join(root, e.Name())
		files, err := os.ReadDir(folder)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") || strings.HasPrefix(f.Name(), ".") {
				continue
			}
			path := filepath.Join(folder, f.Name())
			if strings.EqualFold(ompCwd(path), cleanDir) {
				if info, err := f.Info(); err == nil {
					all = append(all, Found{Path: path, Changed: info.ModTime()})
				}
			}
		}
	}
	slices.SortFunc(all, func(a, b Found) int {
		return b.Changed.Compare(a.Changed)
	})
	return all
}

func OMPConversationID(path string) string {
	if m := ompUUID.FindStringSubmatch(path); len(m) > 1 {
		return m[1]
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if json.Unmarshal(scanner.Bytes(), &rec) == nil && rec.Type == "session" && rec.ID != "" {
			return rec.ID
		}
	}
	return ""
}

func ompCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec struct {
			Type string `json:"type"`
			CWD  string `json:"cwd"`
		}
		if json.Unmarshal(scanner.Bytes(), &rec) == nil && rec.Type == "session" {
			return filepath.Clean(rec.CWD)
		}
	}
	return ""
}

// NewOMPReader follows an OMP transcript.
func NewOMPReader(path string) *Reader {
	return newReader(path, &ompParser{})
}

type ompParser struct{}

func (*ompParser) fresh() parser { return &ompParser{} }

type ompRecord struct {
	Type      string    `json:"type"`
	Model     string    `json:"model"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type ompContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
}

func (p *ompParser) parse(line []byte, r *Reader) {
	var rec ompRecord
	if json.Unmarshal(line, &rec) != nil {
		return
	}
	if rec.Type == "model_change" && rec.Model != "" {
		r.Model = rec.Model
		return
	}
	if rec.Type != "message" {
		return
	}
	var text string
	if json.Unmarshal(rec.Message.Content, &text) == nil {
		p.addText(r, rec.Message.Role, text, rec.Timestamp)
		return
	}
	var blocks []ompContentBlock
	if json.Unmarshal(rec.Message.Content, &blocks) == nil {
		var sb strings.Builder
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				sb.WriteString(b.Text)
			}
		}
		if sb.Len() > 0 {
			p.addText(r, rec.Message.Role, sb.String(), rec.Timestamp)
		}
	}
}

func (p *ompParser) addText(r *Reader, role, text string, at time.Time) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	kind := proto.TranscriptAssistant
	if role == "user" {
		kind = proto.TranscriptUser
	}
	r.add(proto.TranscriptEntry{
		Kind: kind,
		Time: at,
		Text: text,
	})
}
