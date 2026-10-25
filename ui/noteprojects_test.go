package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A page attached from its menu to a project of the host shows its badge,
// and the tree grouped by project lists it there; a project gone from its
// host is marked so.
func TestNoteAttachedToAProject(t *testing.T) {
	h := newNotesHarness(t)
	dir := filepath.Join(t.TempDir(), "antares")
	os.MkdirAll(dir, 0o755)
	if _, err := h.cl.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	info, _ := h.cl.Info()
	h.app.daemonInfo = info
	n, _ := h.cl.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Launch plan"}, Body: "- [ ] ship\n"})
	h.settle("the catalog", func() bool {
		cats, _ := h.cl.Catalogs()
		return len(cats) == 1 && len(cats[0].Projects) == 1
	})
	h.settle("the page", func() bool { return h.tt.HasText("Launch plan") })
	if err := h.tt.Click("Launch plan"); err != nil {
		t.Fatal(err)
	}
	h.settle("the page open", func() bool { return h.tt.HasText("Attach to a project") })
	if err := h.tt.Click("Attach to a project"); err != nil {
		t.Fatal(err)
	}
	h.settle("the dialog", func() bool { return h.tt.HasText("antares") && h.app.attaching != nil })
	if err := h.tt.Click("antares"); err != nil {
		t.Fatal(err)
	}
	if err := h.tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	h.settle("attached", func() bool {
		got, err := h.cl.Note(n.ID)
		return err == nil && len(got.Projects) == 1 && h.tt.HasText("antares · ")
	})
	h.tt.Advance(time.Second)
	shot(t, h.tt, "note-projects.png")

	h.app.notes.ShowProject(byProjectRef(h))
	h.settle("the filter", func() bool { return h.tt.HasText("Launch plan") })

	// Gone from its host: marked, offering to attach it again.
	got, _ := h.cl.Note(n.ID)
	h.cl.RemoveProject(dir)
	h.settle("broken", func() bool {
		h.app.catalogs.read = time.Time{}
		h.app.loadCatalogs()
		return h.app.projectState(got.Projects[0]) == "broken" && h.tt.HasText("broken")
	})
}

func byProjectRef(h *notesHarness) string {
	for _, n := range h.app.noteList.list {
		if len(n.Projects) > 0 {
			return n.Projects[0].Ref
		}
	}
	return ""
}

var _ = ui.Center
var _ = theme.Border
