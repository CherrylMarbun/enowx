package usersync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type pair struct {
	t    *testing.T
	a, b *Folder
	base *Base
}

func newPair(t *testing.T) *pair {
	a, b := OpenFolder(t.TempDir()), OpenFolder(t.TempDir())
	a.Name, b.Name = "Mac", "vps"
	return &pair{t: t, a: a, b: b, base: &Base{Peer: "vps"}}
}

func (p *pair) put(f *Folder, path, content string) {
	p.t.Helper()
	file := filepath.Join(f.dir, filepath.FromSlash(path))
	os.MkdirAll(filepath.Dir(file), 0o700)
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		p.t.Fatal(err)
	}
}

func (p *pair) get(f *Folder, path string) string {
	data, err := os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(path)))
	if err != nil {
		return "<none>"
	}
	return string(data)
}

func (p *pair) sync() Report {
	p.t.Helper()
	rep, err := Sync(p.a, p.b, p.base)
	if err != nil {
		p.t.Fatal(err)
	}
	return rep
}

func note(title, body string) string {
	return "---\ntitle: " + title + "\norder: 0\nupdated: 2026-10-01T00:00:00Z\n---\n\n" + body
}

func TestSyncBothWays(t *testing.T) {
	p := newPair(t)
	p.put(p.a, "notes/aaaaaaaaaaaa.md", note("Mac's", "one\ntwo\nthree\n"))
	p.put(p.b, "notes/bbbbbbbbbbbb.md", note("vps's", "two\n"))
	p.put(p.b, "notes/proposals.json", "[]") // stays on its host
	if rep := p.sync(); rep.Sent != 1 || rep.Received != 1 {
		t.Fatalf("first sync = %+v", rep)
	}
	if p.get(p.b, "notes/aaaaaaaaaaaa.md") != note("Mac's", "one\ntwo\nthree\n") || p.get(p.a, "notes/bbbbbbbbbbbb.md") != note("vps's", "two\n") {
		t.Fatal("notes didn't cross")
	}
	if p.get(p.a, "notes/proposals.json") != "<none>" {
		t.Error("proposals synced")
	}

	// Changed on both, apart: merged.
	p.put(p.a, "notes/aaaaaaaaaaaa.md", note("Mac's", "ONE\ntwo\nthree\n"))
	p.put(p.b, "notes/aaaaaaaaaaaa.md", note("Mac's", "one\ntwo\nthree\nfrom vps\n"))
	if rep := p.sync(); rep.Merged != 1 {
		t.Fatalf("merge sync = %+v", rep)
	}
	want := note("Mac's", "ONE\ntwo\nthree\nfrom vps\n")
	if p.get(p.a, "notes/aaaaaaaaaaaa.md") != want || p.get(p.b, "notes/aaaaaaaaaaaa.md") != want {
		t.Fatalf("merged %q / %q", p.get(p.a, "notes/aaaaaaaaaaaa.md"), p.get(p.b, "notes/aaaaaaaaaaaa.md"))
	}

	// Deleted on one: deleted on the other, kept in its trash.
	os.Remove(filepath.Join(p.b.dir, "notes", "bbbbbbbbbbbb.md"))
	p.sync()
	if p.get(p.a, "notes/bbbbbbbbbbbb.md") != "<none>" {
		t.Fatal("deletion didn't cross")
	}
	trash, _ := p.a.Trash()
	if len(trash) != 1 || trash[0].Title != "vps's" {
		t.Fatalf("trash = %+v", trash)
	}
	if err := p.a.Restore(trash[0].ID); err != nil {
		t.Fatal(err)
	}
	p.sync()
	if p.get(p.b, "notes/bbbbbbbbbbbb.md") == "<none>" {
		t.Fatal("restored note didn't come back to the other")
	}

	// Nothing more to do: nothing moves.
	if rep := p.sync(); rep.Sent+rep.Received+rep.Merged != 0 {
		t.Errorf("idle sync = %+v", rep)
	}
}

func TestSyncConflictKeepsBoth(t *testing.T) {
	p := newPair(t)
	p.put(p.a, "notes/aaaaaaaaaaaa.md", note("Plan", "step\n"))
	p.sync()
	p.put(p.a, "notes/aaaaaaaaaaaa.md", note("Plan", "mac step\n"))
	p.put(p.b, "notes/aaaaaaaaaaaa.md", note("Plan", "vps step\n"))
	if rep := p.sync(); rep.Conflicts != 1 {
		t.Fatalf("sync = %+v", rep)
	}
	if p.get(p.b, "notes/aaaaaaaaaaaa.md") != note("Plan", "mac step\n") {
		t.Error("the first's version isn't in place on both")
	}
	for _, f := range []*Folder{p.a, p.b} {
		cs, _ := f.Conflicts()
		if len(cs) != 1 || cs[0].Title != "Plan" || len(cs[0].Versions) != 2 {
			t.Fatalf("conflicts = %+v", cs)
		}
	}
	cs, _ := p.b.Conflicts()
	if err := p.b.Resolve(cs[0].ID, "both"); err != nil {
		t.Fatal(err)
	}
	p.sync()
	files, _ := p.a.Files()
	var copies int
	for _, f := range files {
		if strings.Contains(p.get(p.a, f.Path), "title: Plan (vps)") {
			copies++
		}
	}
	if left, _ := p.a.Conflicts(); len(left) != 0 || copies != 1 {
		t.Errorf("after keeping both: %d conflicts, %d copies", len(left), copies)
	}
}

func TestSyncDedupesTheSameNote(t *testing.T) {
	p := newPair(t)
	p.put(p.a, "notes/aaaaaaaaaaaa.md", note("TODO", "- [ ] x\n"))
	p.put(p.b, "notes/bbbbbbbbbbbb.md", note("TODO", "- [ ] x\n"))
	if rep := p.sync(); rep.Deduped != 1 {
		t.Fatalf("sync = %+v", rep)
	}
	fa, _ := p.a.Files()
	fb, _ := p.b.Files()
	if len(fa) != 1 || len(fb) != 1 {
		t.Errorf("files: %v / %v", fa, fb)
	}
}
