package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

func TestTranscriptFollowsTheSessionsFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	// A stand-in for Claude Code: what matters is the session's agent and
	// folder.
	sess, err := srv.sessions.Create(session.Spec{Command: []string{"sleep", "30"}, Dir: dir, Meta: session.Meta{Agent: "claude"}})
	if err != nil {
		t.Fatal(err)
	}
	shell, err := srv.sessions.Create(session.Spec{Command: []string{"sleep", "30"}, Dir: dir})
	if err != nil {
		t.Fatal(err)
	}

	got, err := c.Transcript(sess.ID(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Supported || len(got.Entries) != 0 {
		t.Fatalf("before any record: %+v", got)
	}
	if got, _ := c.Transcript(shell.ID(), 0); got.Supported {
		t.Fatalf("a shell session has a transcript: %+v", got)
	}

	folder := filepath.Join(home, ".claude", "projects", claudeFolderOf(dir))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	// A transcript from before the session isn't its own.
	old := filepath.Join(folder, "old.jsonl")
	if err := os.WriteFile(old, []byte(`{"type":"user","message":{"content":"old"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "new.jsonl"), []byte(`{"type":"user","message":{"content":"hello"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(lookEvery + 100*time.Millisecond)
	got, err = c.Transcript(sess.ID(), got.Rev)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Reset || len(got.Entries) != 1 || got.Entries[0].Text != "hello" {
		t.Fatalf("after the agent recorded: %+v", got)
	}
	again, _ := c.Transcript(sess.ID(), got.Rev)
	if len(again.Entries) != 0 || again.Reset {
		t.Fatalf("nothing changed, yet: %+v", again)
	}
}

func TestUnshownTranscriptIsDroppedAndReadAnew(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	sess, err := srv.sessions.Create(session.Spec{Command: []string{"sleep", "30"}, Dir: dir, Meta: session.Meta{Agent: "claude"}})
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(home, ".claude", "projects", claudeFolderOf(dir))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []byte
	for range 10 {
		lines = append(lines, `{"type":"user","message":{"content":"hi"}}`+"\n"...)
	}
	if err := os.WriteFile(filepath.Join(folder, "s.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	ask := func(since int64) proto.Transcript {
		got, err := c.TranscriptPage(proto.TranscriptParams{ID: sess.ID(), Since: since, Tail: 3})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	got := ask(0)
	if len(got.Entries) != 3 {
		t.Fatalf("first page: %d entries", len(got.Entries))
	}

	// Nobody asked for a while: the next chat to ask drops it.
	srv.transcripts.mu.Lock()
	srv.transcripts.byID[sess.ID()].used = time.Now().Add(-dropAfter - time.Second)
	srv.transcripts.mu.Unlock()
	other, err := srv.sessions.Create(session.Spec{Command: []string{"sleep", "30"}, Dir: t.TempDir(), Meta: session.Meta{Agent: "claude"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Transcript(other.ID(), 0); err != nil {
		t.Fatal(err)
	}
	srv.transcripts.mu.Lock()
	_, kept := srv.transcripts.byID[sess.ID()]
	srv.transcripts.mu.Unlock()
	if kept {
		t.Fatal("an unshown transcript stays in memory")
	}

	// Shown again, it starts over with the last page, not every entry.
	again := ask(got.Rev)
	if !again.Reset || len(again.Entries) != 3 {
		t.Fatalf("shown again: reset %v, %d entries", again.Reset, len(again.Entries))
	}
}

func claudeFolderOf(dir string) string {
	out := []byte(dir)
	for i, b := range out {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			out[i] = '-'
		}
	}
	return string(out)
}
