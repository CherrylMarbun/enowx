package notes

import (
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Groups hold top-level notes; renaming one moves its notes with it,
// deleting one leaves them without a group, never gone.
func TestGroupsHoldNotes(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "notes"))
	todo, err := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Ship", Group: "TODO"}})
	if err != nil {
		t.Fatal(err)
	}
	idea, _ := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Debloater"}})
	sub, _ := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "MVP", Parent: idea.ID, Group: "TODO"}})
	if sub.Group != "" {
		t.Fatalf("a page under another got a group: %+v", sub)
	}
	if err := s.SaveGroup("", proto.NoteGroup{Name: "Ideas", Icon: "lucide:lightbulb"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveGroup("", proto.NoteGroup{Name: "Ideas"}); err == nil {
		t.Fatal("two groups of one name")
	}
	if _, err := s.SetGroup(idea.ID, "Ideas"); err != nil {
		t.Fatal(err)
	}
	// A page moved out from under another goes to the top, in the group.
	if n, err := s.SetGroup(sub.ID, "Ideas"); err != nil || n.Parent != "" || n.Group != "Ideas" {
		t.Fatalf("moved %+v, %v", n, err)
	}
	if err := s.MoveGroup("Ideas", "TODO"); err != nil {
		t.Fatal(err)
	}
	groups, _ := s.Groups()
	if len(groups) != 2 || groups[0].Name != "Ideas" || groups[0].Icon != "lucide:lightbulb" || groups[1].Name != "TODO" {
		t.Fatalf("groups %+v", groups)
	}
	if err := s.SaveGroup("Ideas", proto.NoteGroup{Name: "Projects", Icon: "lucide:lightbulb", Folded: true}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Get(idea.ID); n.Group != "Projects" {
		t.Fatalf("after renaming: %+v", n.NoteInfo)
	}
	// Saving keeps a note's group.
	if n, _ := s.Save(proto.Note{NoteInfo: proto.NoteInfo{ID: idea.ID, Title: "Debloater app"}}); n.Group != "Projects" {
		t.Fatalf("after saving: %+v", n)
	}
	if err := s.DeleteGroup("TODO"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Get(todo.ID); err != nil || n.Group != "" {
		t.Fatalf("after deleting its group: %+v, %v", n.NoteInfo, err)
	}
	// Dropped before a note of a group, a note joins it.
	if err := s.Move(todo.ID, "", idea.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Get(todo.ID); n.Group != "Projects" {
		t.Fatalf("after moving: %+v", n.NoteInfo)
	}
}
