package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestArrangeKeepsOrderAndGroupsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "state", "projects.json")
	s, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, name := range []string{"a", "b", "c", "d"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		info, err := s.Add(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(name, err)
		}
		paths = append(paths, info.Path)
	}
	a, b, c, d := paths[0], paths[1], paths[2], paths[3]

	// d goes first, in group "work"; b joins it after d.
	if err := s.Arrange(d, "work", a); err != nil {
		t.Fatal(err)
	}
	if err := s.Arrange(b, "work", a); err != nil {
		t.Fatal(err)
	}
	want := func(order []string, groups map[string]string) {
		t.Helper()
		s, err := Open(file) // what a restarted daemon reads
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range s.List() {
			got = append(got, p.Path)
			if p.Group != groups[p.Path] {
				t.Errorf("%s in group %q, want %q", filepath.Base(p.Path), p.Group, groups[p.Path])
			}
		}
		if !slices.Equal(got, order) {
			t.Errorf("order %v, want %v", got, order)
		}
	}
	want([]string{d, b, a, c}, map[string]string{d: "work", b: "work"})

	// The group moves last as a whole, in its order.
	if err := s.MoveGroup("work", ""); err != nil {
		t.Fatal(err)
	}
	want([]string{a, c, d, b}, map[string]string{d: "work", b: "work"})

	if err := s.RenameGroup("work", "clients"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(b); err != nil {
		t.Fatal(err)
	}
	want([]string{a, c, d}, map[string]string{d: "clients"})

	if err := s.RenameGroup("clients", ""); err != nil {
		t.Fatal(err)
	}
	want([]string{a, c, d}, nil)
}
