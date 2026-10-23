package chatview

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestTurnsFoldTheWorkAndKeepTheAnswer(t *testing.T) {
	entries := []proto.TranscriptEntry{
		{Index: 0, Kind: proto.TranscriptUser, Text: "fix it"},
		{Index: 1, Kind: proto.TranscriptThinking, Text: "hmm"},
		{Index: 2, Kind: proto.TranscriptAssistant, Text: "Reading the code."},
		{Index: 3, Kind: proto.TranscriptTool, Text: "Edit", Added: 2, Removed: 1},
		{Index: 4, Kind: proto.TranscriptAssistant, Text: "Fixed."},
		{Index: 5, Kind: proto.TranscriptNotice, Text: "Interrupted"},
		{Index: 6, Kind: proto.TranscriptUser, Text: "thanks"},
	}
	turns := splitTurns(entries)
	if len(turns) != 3 {
		t.Fatalf("turns = %+v", turns)
	}
	first := turns[0]
	if first.prompt == nil || len(first.work) != 3 || len(first.answer) != 1 || first.answer[0].Text != "Fixed." {
		t.Fatalf("first turn = %+v", first)
	}
	if turns[1].prompt != nil || turns[1].answer[0].Kind != proto.TranscriptNotice {
		t.Fatalf("notice turn = %+v", turns[1])
	}
	if turns[2].prompt == nil || turns[2].prompt.Text != "thanks" {
		t.Fatalf("last turn = %+v", turns[2])
	}
}

func TestToolRunsSumUpByWhatTheyDid(t *testing.T) {
	bash := func(cmd string) proto.TranscriptEntry {
		return proto.TranscriptEntry{Kind: proto.TranscriptTool, Text: "Bash", Detail: cmd}
	}
	tools := []proto.TranscriptEntry{
		bash("cat a.go"), bash("cd web && sed -n '1,80p' index.html"), bash("pwd && rg --files"),
		bash("FOO=1 go test ./..."), {Kind: proto.TranscriptTool, Text: "Edit"},
	}
	if got := summary(tools); got != "Read 2 files, searched once, edited 1 file, ran 1 command" {
		t.Fatalf("summary = %q", got)
	}
	steps := workSteps(append([]proto.TranscriptEntry{{Kind: proto.TranscriptAssistant, Text: "Looking."}}, tools...))
	if len(steps) != 2 || steps[0].entry == nil || len(steps[1].tools) != 5 {
		t.Fatalf("steps = %+v", steps)
	}
}

// Editing a message goes back over it and every message of the user's
// after it.
func TestEditGoesBackOverTheMessagesAfter(t *testing.T) {
	ch := New()
	ch.entries = []proto.TranscriptEntry{
		{Index: 0, Kind: proto.TranscriptUser, Text: "one"},
		{Index: 1, Kind: proto.TranscriptAssistant, Text: "1"},
		{Index: 2, Kind: proto.TranscriptUser, Text: "two"},
		{Index: 3, Kind: proto.TranscriptAssistant, Text: "2"},
		{Index: 4, Kind: proto.TranscriptUser, Text: "three"},
	}
	ch.edit(ch.entries[2])
	if ch.input != "two" || ch.editBack() != 2 {
		t.Fatalf("input %q, back %d", ch.input, ch.editBack())
	}
	var back int
	var original, sent string
	ch.send(Options{Edit: func(b int, o, text string, _ []string) { back, original, sent = b, o, text }}, "two, changed")
	if back != 2 || original != "two" || sent != "two, changed" || ch.editing != nil {
		t.Fatalf("edit(%d, %q, %q), editing %v", back, original, sent, ch.editing)
	}
}
