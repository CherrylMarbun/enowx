package daemon

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// waitImport follows import id's events until it ends.
func waitImport(t *testing.T, events <-chan proto.NoteImportStatus, id string) proto.NoteImportStatus {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case st := <-events:
			if st.ID == id && st.State != proto.ImportRunning {
				return st
			}
		case <-deadline:
			t.Fatal("the import never ended")
		}
	}
}

func importEvents(t *testing.T, srv *Server) (*client.Client, <-chan proto.NoteImportStatus) {
	events := make(chan proto.NoteImportStatus, 64)
	c := connect(t, srv, func(e proto.Event) {
		var st proto.NoteImportStatus
		if e.Name == proto.EventNoteImport && json.Unmarshal(e.Data, &st) == nil {
			events <- st
		}
	})
	return c, events
}

// A file of the app's machine is staged on the host, previewed, then
// imported: its notes made, the staged file gone. Exported for the app,
// the notes come back as a backup another host restores.
func TestImportAndExportNotes(t *testing.T) {
	srv := startServer(t)
	c, events := importEvents(t, srv)
	local := filepath.Join(t.TempDir(), "Plan.md")
	os.WriteFile(local, []byte("# Plan\n\n- [ ] ship\n"), 0o644)
	staged, err := c.StageNotes(local)
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.ImportNotes(proto.NoteImportParams{Path: staged, DryRun: true, Group: "Work"})
	if err != nil {
		t.Fatal(err)
	}
	st = waitImport(t, events, st.ID)
	if st.State != proto.ImportDone || st.Preview == nil || len(st.Preview.Notes) != 1 || st.Preview.Notes[0].Group != "Work" {
		t.Fatalf("dry run %+v", st)
	}
	if list, _ := c.Notes(); len(list) != 0 {
		t.Fatal("a dry run made notes")
	}
	st, _ = c.ImportNotes(proto.NoteImportParams{Path: staged, Group: "Work"})
	st = waitImport(t, events, st.ID)
	if st.State != proto.ImportDone || st.Report == nil || st.Report.Imported != 1 {
		t.Fatalf("import %+v", st)
	}
	list, _ := c.Notes()
	if len(list) != 1 || list[0].Title != "Plan" || list[0].Group != "Work" {
		t.Fatalf("notes %+v", list)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Error("the staged file stayed")
	}

	out, err := c.ExportNotes(proto.NoteExportParams{All: true, Format: proto.ExportJSON})
	if err != nil || out.Name != "enowx-notes.json" || out.Notes != 1 {
		t.Fatalf("export %+v, %v", out, err)
	}
	var data bytes.Buffer
	if err := c.ReadExport(out.Path, &data); err != nil || int64(data.Len()) != out.Size {
		t.Fatalf("read %d of %d bytes: %v", data.Len(), out.Size, err)
	}
	if err := c.ReadExport(filepath.Join(srv.stagingDir(), "..", "settings.json"), &bytes.Buffer{}); err == nil {
		t.Error("read a file that isn't an export")
	}
	backup := filepath.Join(t.TempDir(), out.Name)
	os.WriteFile(backup, data.Bytes(), 0o644)

	other := startServer(t)
	c2, events2 := importEvents(t, other)
	staged, _ = c2.StageNotes(backup)
	st, _ = c2.ImportNotes(proto.NoteImportParams{Path: staged})
	if st = waitImport(t, events2, st.ID); st.State != proto.ImportDone {
		t.Fatalf("restore %+v", st)
	}
	got, _ := c2.Notes()
	if len(got) != 1 || got[0].ID != list[0].ID || got[0].Group != "Work" {
		t.Fatalf("restored %+v, want %+v", got, list)
	}
}

// An export to a folder on the host lands there, named for what it holds.
func TestExportNotesToTheHost(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	n, _ := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Ideas"}, Body: "**bold**\n"})
	dir := t.TempDir()
	out, err := c.ExportNotes(proto.NoteExportParams{IDs: []string{n.ID}, Format: proto.ExportHTML, Out: dir})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Ideas.html"))
	if err != nil || out.Path != filepath.Join(dir, "Ideas.html") || !strings.Contains(string(data), "<strong>bold</strong>") {
		t.Fatalf("export %+v: %v", out, err)
	}
	if _, err := c.ImportNotes(proto.NoteImportParams{Path: "relative/path"}); err == nil {
		t.Error("imported from a relative path")
	}
}
