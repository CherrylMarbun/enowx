package transcript

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestOMPConversationID(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{
			path: "/home/user/.omp/agent/sessions/--test--/2026-10-26T18-26-49-514Z_01a17977-142a-773a-a2e9-566f308b86b7.jsonl",
			want: "01a17977-142a-773a-a2e9-566f308b86b7",
		},
		{
			path: "session_abc-123.jsonl",
			want: "abc-123",
		},
	}
	for _, tc := range cases {
		if got := OMPConversationID(tc.path); got != tc.want {
			t.Errorf("OMPConversationID(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestOMPReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-10-26T18-26-49-514Z_01a17977-142a-773a-a2e9-566f308b86b7.jsonl")
	content := `{"type":"session","id":"01a17977-142a-773a-a2e9-566f308b86b7","cwd":"/repo"}
{"type":"model_change","model":"my-model"}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Halo"}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Thinking..."},{"type":"text","text":"Halo juga!"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewOMPReader(path)
	if err := r.Update(); err != nil {
		t.Fatal(err)
	}
	if r.Model != "my-model" {
		t.Errorf("Model = %q, want %q", r.Model, "my-model")
	}
	if r.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", r.Len())
	}
	entries, _ := r.Since(0)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if got := entries[0]; got.Kind != proto.TranscriptUser || got.Text != "Halo" {
		t.Errorf("Entry[0] = %+v", got)
	}
	if got := entries[1]; got.Kind != proto.TranscriptAssistant || got.Text != "Halo juga!" {
		t.Errorf("Entry[1] = %+v", got)
	}
}
