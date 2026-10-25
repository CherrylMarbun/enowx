package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestNotesNestMoveAndDelete(t *testing.T) {
	dir := t.TempDir()
	s := Open(filepath.Join(dir, "notes"))
	save := func(n proto.Note) proto.NoteInfo {
		t.Helper()
		info, err := s.Save(n)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	ideas := save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Ideas", Icon: "lucide:lightbulb"}, Body: "# Ideas\n\n- [ ] one\n"})
	todo := save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Todo"}})
	sub := save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Sub", Parent: ideas.ID}, Body: "under ideas"})
	if _, err := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "x", Parent: "000000000000"}}); err == nil {
		t.Fatal("a note under a note that isn't")
	}

	// The file is Markdown any editor reads.
	data, err := os.ReadFile(filepath.Join(dir, "notes", ideas.ID+".md"))
	if err != nil || !strings.Contains(string(data), "title: Ideas\n") || !strings.HasSuffix(string(data), "# Ideas\n\n- [ ] one\n") {
		t.Fatalf("file %q, %v", data, err)
	}
	got, err := s.Get(ideas.ID)
	if err != nil || got.Body != "# Ideas\n\n- [ ] one\n" || got.Icon != "lucide:lightbulb" {
		t.Fatalf("got %+v, %v", got, err)
	}
	// Saving keeps its place and creation.
	edited := save(proto.Note{NoteInfo: proto.NoteInfo{ID: sub.ID, Title: "Sub page", Parent: ""}, Body: "changed"})
	if edited.Parent != ideas.ID || !edited.Created.Equal(sub.Created) {
		t.Fatalf("edited %+v", edited)
	}

	order := func() string {
		list, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		var b []string
		for _, n := range list {
			p := "-"
			if n.Parent != "" {
				p = n.Parent[:2]
			}
			b = append(b, p+":"+n.Title)
		}
		return strings.Join(b, " ")
	}
	if err := s.Move(todo.ID, "", ideas.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "-:Todo -:Ideas "+ideas.ID[:2]+":Sub page" {
		t.Fatalf("after moving Todo first: %s", got)
	}
	if err := s.Move(ideas.ID, sub.ID, ""); err == nil {
		t.Fatal("a note moved under its own child")
	}
	if err := s.Move(sub.ID, todo.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(todo.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "-:Ideas" {
		t.Fatalf("after deleting Todo and what's under it: %s", got)
	}
	if _, err := s.Get("../../etc"); err == nil {
		t.Fatal("read outside the folder")
	}
}

func TestVersionsComeBack(t *testing.T) {
	s := Open(t.TempDir())
	n, err := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Plan"}, Body: "first\n"})
	if err != nil {
		t.Fatal(err)
	}
	full, _ := s.Get(n.ID)
	full.Body = "second\n"
	s.Save(full) // kept: the first version
	full.Body = "third\n"
	s.Save(full) // within versionEvery: not kept again
	hist, err := s.History(n.ID)
	if err != nil || len(hist) != 1 {
		t.Fatalf("history %+v, %v", hist, err)
	}
	if _, err := s.RestoreVersion(n.ID, hist[0].ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(n.ID)
	hist, _ = s.History(n.ID)
	if got.Body != "first\n" || len(hist) != 2 {
		t.Fatalf("after restoring: %q, %d versions", got.Body, len(hist))
	}
	if v, _ := s.Version(n.ID, hist[0].ID); v.Body != "third\n" {
		t.Errorf("the version kept before restoring: %q", v.Body)
	}
}
