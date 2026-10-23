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
