package notesio

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// importInto reads p and writes it into a new store.
func importInto(t *testing.T, s *notes.Store, p string, o Options, a ApplyOptions) (*Batch, proto.NoteImportReport) {
	t.Helper()
	b, err := Read(context.Background(), p, o)
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.Apply(context.Background(), s, a)
	if err != nil {
		t.Fatal(err)
	}
	return b, r
}

// byTitle is the store's notes by title, with their bodies.
func byTitle(t *testing.T, s *notes.Store) map[string]proto.Note {
	t.Helper()
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]proto.Note{}
	for _, n := range list {
		full, err := s.Get(n.ID)
		if err != nil {
			t.Fatal(err)
		}
		out[n.Title] = full
	}
	return out
}

func contains(t *testing.T, where, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("%s lacks %q:\n%s", where, w, body)
		}
	}
}

// A Notion export, zipped as Notion hands it out: pages nested as they
// were, a database a table whose row with a page of its own links it, the
// callout, to-dos and image kept, links between pages by title.
func TestImportNotionExport(t *testing.T) {
	s := notes.Open(t.TempDir())
	p := zipOf(t, "Export-abc.zip", notionExport(t))
	b, r := importInto(t, s, p, Options{}, ApplyOptions{Group: "Work"})
	if b.From != proto.ImportNotion {
		t.Fatalf("read as %q", b.From)
	}
	got := byTitle(t, s)
	plan, ok := got["Project Plan"]
	if !ok {
		t.Fatalf("no Project Plan in %v", got)
	}
	if plan.Group != "Work" || plan.Parent != "" {
		t.Errorf("plan in group %q under %q", plan.Group, plan.Parent)
	}
	contains(t, "plan", plan.Body, "> 💡 Keep it small.", "- [x] Write the spec", "- [ ] Ship it",
		"[[Meeting notes]]", "[[Roadmap]]", "| API | Ada |", "](assets/"+plan.ID+"/diagram.png)")
	if strings.HasPrefix(plan.Body, "# Project Plan") {
		t.Error("the title is in the body too")
	}
	if data, err := s.Asset("assets/" + plan.ID + "/diagram.png"); err != nil || !bytes.Equal(data, pngBytes(t)) {
		t.Errorf("image: %v", err)
	}
	meeting, road := got["Meeting notes"], got["Roadmap"]
	if meeting.Parent != plan.ID || road.Parent != plan.ID {
		t.Errorf("meeting under %q, roadmap under %q, want %q", meeting.Parent, road.Parent, plan.ID)
	}
	contains(t, "meeting", meeting.Body, "[[Project Plan|plan]]")
	contains(t, "roadmap", road.Body, "| Name | Status | Owner |", "| [[Login]] | Done | Ada |", "| Search | Planned | Grace |")
	login, ok := got["Login"]
	if !ok || login.Parent != road.ID {
		t.Fatalf("Login: %+v", login.NoteInfo)
	}
	contains(t, "login", login.Body, "Use passkeys first.", "Properties: Status: Done · Owner: Ada")
	if _, ok := got["Search"]; ok {
		t.Error("a row with nothing but its properties became a page")
	}
	if r.Imported != 4 {
		t.Errorf("imported %d, want 4", r.Imported)
	}
}

// A database as a note per row, when asked.
func TestImportNotionRowsAsNotes(t *testing.T) {
	s := notes.Open(t.TempDir())
	importInto(t, s, writeTree(t, notionExport(t)), Options{RowsAsNotes: true}, ApplyOptions{})
	got := byTitle(t, s)
	road := got["Roadmap"]
	if got["Search"].Parent != road.ID || got["Login"].Parent != road.ID {
		t.Fatalf("rows not under the database: %+v", got)
	}
	contains(t, "roadmap", road.Body, "- [[Login]]", "- [[Search]]")
}

// An Obsidian vault: folders at its top groups, deeper ones pages with
// their notes, the folder note their page; wikilinks with aliases and
// headings, links to files and attachments by name resolved; code kept.
func TestImportObsidianVault(t *testing.T) {
	s := notes.Open(t.TempDir())
	b, r := importInto(t, s, zipOf(t, "Vault.zip", obsidianVault(t)), Options{}, ApplyOptions{})
	if b.From != proto.ImportObsidian {
		t.Fatalf("read as %q", b.From)
	}
	got := byTitle(t, s)
	home, alpha, sub, daily := got["Home"], got["Alpha"], got["Notes"], got["2024-01-01"]
	if home.Group != "" || alpha.Group != "Projects" || daily.Group != "Daily" {
		t.Errorf("groups: home %q, alpha %q, daily %q", home.Group, alpha.Group, daily.Group)
	}
	if sub.Parent != alpha.ID {
		t.Errorf("Notes is under %q, not Alpha's folder note", sub.Parent)
	}
	contains(t, "home", home.Body, "Tags: start", "[[Alpha|the Alpha project]]", "[[Notes]]", "](assets/"+home.ID+"/cat.png)", "// [[not a link]]")
	contains(t, "alpha", alpha.Body, "[[Home#Intro]]")
	contains(t, "daily", daily.Body, "[[Home]]", "[[Alpha|the plan]]")
	if alpha.Created.Format("2006-01-02") != "2024-03-01" {
		t.Errorf("alpha created %v", alpha.Created)
	}
	if r.Imported != 4 {
		t.Errorf("imported %d, want 4", r.Imported)
	}
}

// Evernote: to-dos, bold, an image from its resources; its script gone.
func TestImportEnex(t *testing.T) {
	img := pngBytes(t)
	sum := md5.Sum(img)
	src := strings.ReplaceAll(strings.ReplaceAll(enex, "{{HASH}}", hex.EncodeToString(sum[:])), "{{DATA}}", base64.StdEncoding.EncodeToString(img))
	p := filepath.Join(t.TempDir(), "Home.enex")
	os.WriteFile(p, []byte(src), 0o644)
	s := notes.Open(t.TempDir())
	importInto(t, s, p, Options{}, ApplyOptions{})
	n := byTitle(t, s)["Groceries"]
	if n.Group != "Home" {
		t.Errorf("group %q", n.Group)
	}
	contains(t, "groceries", n.Body, "Tags: home", "**Buy** these:", "- [x] Milk", "- [ ] Bread", "](assets/"+n.ID+"/receipt.png)")
	if strings.Contains(n.Body, "alert") {
		t.Error("the script came along")
	}
	if n.Created.Format("2006-01-02") != "2023-12-01" {
		t.Errorf("created %v", n.Created)
	}
}

// Google Keep's Takeout: a checklist with its labels.
func TestImportKeep(t *testing.T) {
	dir := writeTree(t, map[string][]byte{"Takeout/Keep/Packing.json": []byte(keepJSON), "Takeout/Keep/Labels.txt": []byte("Travel")})
	s := notes.Open(t.TempDir())
	b, _ := importInto(t, s, dir, Options{}, ApplyOptions{})
	if b.From != proto.ImportKeep {
		t.Fatalf("read as %q", b.From)
	}
	contains(t, "packing", byTitle(t, s)["Packing"].Body, "Tags: Travel", "- [x] Passport", "- [ ] Charger")
}

// HTML (Apple Notes' exports): structure kept, nothing that runs, the
// embed reported.
func TestImportHTML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Recipe.html")
	os.WriteFile(p, []byte(appleHTML), 0o644)
	s := notes.Open(t.TempDir())
	b, _ := importInto(t, s, p, Options{}, ApplyOptions{})
	n := byTitle(t, s)["Recipe"]
	contains(t, "recipe", n.Body, "Mix *flour* and water, then [bake](https://example.com/bake).", "1. Mix", "  - at 200°",
		"> Best warm.", "| Item | Grams |", "| Flour | 500 |")
	if strings.Contains(n.Body, "javascript") || strings.HasPrefix(n.Body, "# Recipe") {
		t.Errorf("body:\n%s", n.Body)
	}
	if len(b.Unconverted) != 1 || !strings.Contains(b.Unconverted[0].What, "embedded page") {
		t.Errorf("unconverted %+v", b.Unconverted)
	}
}

// Importing the same source again makes nothing twice: skipped, written
// again, or made anew as asked.
func TestImportAgain(t *testing.T) {
	s := notes.Open(t.TempDir())
	src := writeTree(t, obsidianVault(t))
	importInto(t, s, src, Options{}, ApplyOptions{})
	_, r := importInto(t, s, src, Options{}, ApplyOptions{})
	if r.Imported != 0 || len(r.Skipped) != 4 {
		t.Fatalf("again: %+v", r)
	}
	os.WriteFile(filepath.Join(src, "Vault/Projects/Alpha.md"), []byte("Changed.\n"), 0o644)
	b, err := Read(context.Background(), src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	pv, err := b.Preview(s, ApplyOptions{Mode: proto.ImportOverwrite})
	if err != nil || pv.Notes[0].Action != proto.ImportOverwrite {
		t.Fatalf("preview %+v %v", pv, err)
	}
	if r, err = b.Apply(context.Background(), s, ApplyOptions{Mode: proto.ImportOverwrite}); err != nil || r.Overwritten != 4 {
		t.Fatalf("overwrite: %+v %v", r, err)
	}
	if got := byTitle(t, s); len(got) != 4 || got["Alpha"].Body != "Changed.\n" {
		t.Fatalf("after overwrite: %d notes, Alpha %q", len(got), got["Alpha"].Body)
	}
	if r, _ = b.Apply(context.Background(), s, ApplyOptions{Mode: proto.ImportDuplicate}); r.Imported != 4 {
		t.Fatalf("duplicate: %+v", r)
	}
	if list, _ := s.List(); len(list) != 8 {
		t.Fatalf("%d notes after duplicating, want 8", len(list))
	}
}

// A preview says what would be made, and the titles taken already.
func TestImportPreview(t *testing.T) {
	s := notes.Open(t.TempDir())
	s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Home"}})
	b, err := Read(context.Background(), writeTree(t, obsidianVault(t)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	pv, err := b.Preview(s, ApplyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pv.Notes) != 4 || pv.Assets != 1 || len(pv.Conflicts) != 1 || pv.Conflicts[0] != "Home" {
		t.Fatalf("preview %+v", pv)
	}
	if list, _ := s.List(); len(list) != 1 {
		t.Fatal("a preview wrote notes")
	}
}

// A canceled import takes back what it made.
func TestImportCanceled(t *testing.T) {
	s := notes.Open(t.TempDir())
	b, err := Read(context.Background(), writeTree(t, obsidianVault(t)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_, err = b.Apply(ctx, s, ApplyOptions{Progress: func(done, _ int) {
		if done == 2 {
			cancel()
		}
	}})
	if err == nil {
		t.Fatal("not canceled")
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatalf("%d notes left", len(list))
	}
}
