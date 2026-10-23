package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeTranscriptFollowsTheFile(t *testing.T) {
	home := t.TempDir()
	dir := "/home/me/code/app"
	folder := filepath.Join(home, ".claude", "projects", "-home-me-code-app")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "s1.jsonl")
	appendTo(t, path, `{"type":"user","cwd":"/home/me/code/app","timestamp":"2026-10-22T06:00:00Z","message":{"role":"user","content":"Fix the login bug"}}
{"type":"user","isMeta":true,"message":{"content":"meta"}}
{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Look at auth.go"},{"type":"text","text":"Reading the code."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"go test ./auth"}}]}}
{"type":"assistant","isSidechain":true,"message":{"content":"a subagent's words"}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"FAIL auth"`)

	if got := ClaudeFiles(home, dir); len(got) != 1 || got[0] != path {
		t.Fatalf("files = %v", got)
	}
	r := NewClaudeReader(path)
	if err := r.Update(); err != nil {
		t.Fatal(err)
	}
	entries, rev := r.Since(0)
	kinds := []string{}
	for _, e := range entries {
		kinds = append(kinds, e.Kind)
	}
	want := []string{proto.TranscriptUser, proto.TranscriptThinking, proto.TranscriptAssistant, proto.TranscriptTool}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	if tool := entries[3]; tool.Text != "Bash" || tool.Detail != "go test ./auth" || tool.Done {
		t.Fatalf("tool = %+v", tool)
	}

	// The agent finishes the half-written line, then goes on.
	appendTo(t, path, `,"is_error":true}]}}
{"type":"user","message":{"content":"<command-name>/clear</command-name>"}}
`)
	if err := r.Update(); err != nil {
		t.Fatal(err)
	}
	changed, _ := r.Since(rev)
	if len(changed) != 2 {
		t.Fatalf("changed = %+v", changed)
	}
	if tool := changed[0]; tool.Index != 3 || !tool.Done || !tool.Failed || tool.Result != "FAIL auth" {
		t.Fatalf("tool after result = %+v", tool)
	}
	if n := changed[1]; n.Kind != proto.TranscriptNotice || n.Text != "Ran /clear" {
		t.Fatalf("notice = %+v", n)
	}
}

func TestClaudeFilesFindsFoldersByTheirRecords(t *testing.T) {
	home := t.TempDir()
	folder := filepath.Join(home, ".claude", "projects", "named-differently")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	appendTo(t, filepath.Join(folder, "s.jsonl"), `{"type":"summary"}`+"\n"+`{"type":"user","cwd":"/srv/my.app","message":{"content":"hi"}}`+"\n")
	if got := ClaudeFiles(home, "/srv/my.app"); len(got) != 1 {
		t.Fatalf("files = %v", got)
	}
	if got := ClaudeFiles(home, "/srv/other"); len(got) != 0 {
		t.Fatalf("files for another folder = %v", got)
	}
}

func TestClaudeSettingsThoughtsAndEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	appendTo(t, path, `{"type":"permission-mode","permissionMode":"plan"}
{"type":"assistant","effort":"high","thinkingDurationMs":4200,"message":{"model":"claude-opus-5-5","content":[{"type":"thinking","thinking":"Plan it"},{"type":"tool_use","id":"e1","name":"Edit","input":{"file_path":"a.go","old_string":"a\nb\nc","new_string":"a\nB\nc\nd"}}]}}
{"type":"assistant","message":{"model":"<synthetic>","content":"ok"}}
{"type":"assistant","message":{"model":"claude-opus-5-5","usage":{"input_tokens":2,"cache_read_input_tokens":140126,"cache_creation_input_tokens":367,"output_tokens":920},"content":"done"}}
`)
	r := NewClaudeReader(path)
	if err := r.Update(); err != nil {
		t.Fatal(err)
	}
	if r.Model != "claude-opus-5-5" || r.Mode != "plan" || r.Effort != "high" {
		t.Fatalf("settings = %q %q %q", r.Model, r.Mode, r.Effort)
	}
	if r.Context != 2+140126+367+920 {
		t.Fatalf("context = %d", r.Context)
	}
	entries, _ := r.Since(0)
	if len(entries) < 2 || entries[0].Kind != proto.TranscriptThinking || entries[0].DurationMs != 4200 {
		t.Fatalf("entries = %+v", entries)
	}
	if e := entries[1]; e.Text != "Edit" || e.Added != 2 || e.Removed != 1 || e.Old == "" || e.New == "" {
		t.Fatalf("edit = %+v", e)
	}
}

func TestBeforeStartsAtATurn(t *testing.T) {
	r := &Reader{}
	for _, k := range []string{"user", "assistant", "tool", "user", "assistant", "user", "tool", "assistant"} {
		r.add(proto.TranscriptEntry{Kind: k})
	}
	got, _ := r.Before(r.Len(), 2) // the last two would cut the last turn
	if len(got) != 3 || got[0].Index != 5 {
		t.Fatalf("tail = %+v", got)
	}
	got, _ = r.Before(5, 1)
	if len(got) != 2 || got[0].Index != 3 || got[1].Index != 4 {
		t.Fatalf("before 5 = %+v", got)
	}
	if got, _ := r.Before(0, 10); len(got) != 0 {
		t.Fatalf("before 0 = %+v", got)
	}
}
