package daemon

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// A note saved by one app (or an agent's CLI) reaches the others' lists.
func TestNotesSavedReachEveryApp(t *testing.T) {
	srv := startServer(t)
	events := make(chan []proto.NoteInfo, 4)
	connect(t, srv, func(e proto.Event) {
		var list []proto.NoteInfo
		if e.Name == proto.EventNotesChanged && json.Unmarshal(e.Data, &list) == nil {
			events <- list
		}
	})
	c := connect(t, srv, nil)
	info, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Plan"}, Body: "- [ ] ship"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case list := <-events:
		if len(list) != 1 || list[0].ID != info.ID || list[0].Title != "Plan" {
			t.Fatalf("event %+v", list)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no notes event")
	}
	n, err := c.Note(info.ID)
	if err != nil || n.Body != "- [ ] ship" {
		t.Fatalf("note %+v, %v", n, err)
	}
}
