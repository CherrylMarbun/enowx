package daemon

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/usersync"
	"github.com/enowx/enowx/proto"
)

// Two hosts' notes and groups, synced through their daemons, end up the
// same on both, and the apps of the one changed hear of it.
func TestUserDataSyncsBetweenDaemons(t *testing.T) {
	mac, vps := startServer(t), startServer(t)
	notesOnVPS := make(chan []proto.NoteInfo, 8)
	cv := connect(t, vps, func(e proto.Event) {
		var list []proto.NoteInfo
		if e.Name == proto.EventNotesChanged && json.Unmarshal(e.Data, &list) == nil {
			notesOnVPS <- list
		}
	})
	cm := connect(t, mac, nil)
	todo, err := cm.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "TODO", Group: "Work"}, Body: "- [ ] one\n- [ ] two\n- [ ] three\n"})
	if err != nil {
		t.Fatal(err)
	}
	base := &usersync.Base{}
	sync := func() usersync.Report {
		t.Helper()
		rep, err := usersync.Sync(client.SyncSide{C: cm, From: "vps"}, client.SyncSide{C: cv, From: "mac"}, base)
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}
	if rep := sync(); rep.Sent < 2 {
		t.Fatalf("sync = %+v", rep)
	}
	waitNotes(t, notesOnVPS, func(l []proto.NoteInfo) bool { return len(l) == 1 && l[0].Title == "TODO" && l[0].Group == "Work" })
	groups, _ := cv.NoteGroups()
	if len(groups) != 1 || groups[0].Name != "Work" {
		t.Fatalf("groups on the vps = %+v", groups)
	}

	// Ticked on each side, apart: both ticks on both.
	n, _ := cm.Note(todo.ID)
	n.Body = "- [x] one\n- [ ] two\n- [ ] three\n"
	cm.SaveNote(n)
	n2, _ := cv.Note(todo.ID)
	n2.Body = "- [ ] one\n- [ ] two\n- [x] three\n"
	time.Sleep(1100 * time.Millisecond) // a later "updated"
	cv.SaveNote(n2)
	if rep := sync(); rep.Merged != 1 {
		t.Fatalf("sync = %+v", rep)
	}
	for _, c := range []*client.Client{cm, cv} {
		got, _ := c.Note(todo.ID)
		if got.Body != "- [x] one\n- [ ] two\n- [x] three\n" {
			t.Fatalf("merged body %q", got.Body)
		}
	}
	st, err := cv.SyncStatus()
	if err != nil || st.Files < 2 {
		t.Fatalf("status %+v, %v", st, err)
	}
}

func waitNotes(t *testing.T, ch chan []proto.NoteInfo, ok func([]proto.NoteInfo) bool) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case l := <-ch:
			if ok(l) {
				return
			}
		case <-deadline:
			t.Fatal("notes never came")
		}
	}
}
