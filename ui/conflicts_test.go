package ui

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestConflictNotesNameTheOtherSession(t *testing.T) {
	a := &App{}
	a.sessions = []proto.SessionInfo{{ID: "s1", Name: "rank", Project: "/p"}, {ID: "s2", Name: "docs", Project: "/p"}, {ID: "s3", Name: "ui", Project: "/p"}}
	a.conflicts = map[string][]proto.Conflict{"/p": {
		{File: "a.go", Sessions: []string{"s1", "s2"}, Lines: true},
		{File: "b.go", Sessions: []string{"s2", "s3"}},
	}}
	notes := a.conflictNotes(a.sessions[1])
	if len(notes) != 2 || notes[0].With != "rank" || !notes[0].Lines || notes[1].With != "ui" {
		t.Fatalf("notes of s2 = %+v", notes)
	}
	if len(a.conflictNotes(a.sessions[0])) != 1 {
		t.Fatal("s1 is in one conflict")
	}
}
