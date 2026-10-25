package daemon

import (
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/usersync"
	"github.com/enowx/enowx/proto"
)

// A note's agent asked from another agent's tools: the question is kept
// with the note, its agent picked, and both reach another host by sync.
func TestNoteChatIsKeptAndSynced(t *testing.T) {
	mac, vps := startServer(t), startServer(t)
	cm, cv := connect(t, mac, nil), connect(t, vps, nil)
	n, err := cm.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Plan"}, Body: "- [ ] a\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cm.SetNoteAgent(n.ID, "shell"); err != nil {
		t.Fatal(err)
	}
	log, err := cm.NoteChat(proto.NoteChatParams{ID: n.ID, Prompt: "tidy this up"})
	if err != nil {
		t.Fatal(err)
	}
	if log.Session == "" || len(log.Messages) != 1 || log.Messages[0].Text != "tidy this up" || log.Messages[0].Agent != "shell" {
		t.Fatalf("log %+v", log)
	}
	// Asked again, the conversation running takes it.
	again, err := cm.NoteChat(proto.NoteChatParams{ID: n.ID, Prompt: "and sort it"})
	if err != nil || again.Session != log.Session || len(again.Messages) != 2 {
		t.Fatalf("again %+v, %v", again, err)
	}
	if _, err := usersync.Sync(client.SyncSide{C: cm}, client.SyncSide{C: cv}, &usersync.Base{}); err != nil {
		t.Fatal(err)
	}
	there, err := cv.NoteChatLog(n.ID)
	if err != nil || len(there.Messages) != 2 {
		t.Fatalf("on the other host: %+v, %v", there, err)
	}
	if got, _ := cv.Note(n.ID); got.Agent != "shell" {
		t.Errorf("the note's agent there: %q", got.Agent)
	}
}
