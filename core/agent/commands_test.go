package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandNotesReadFrontMatter(t *testing.T) {
	dir := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "skills", "vps", "SKILL.md"), "---\nname: vps\ndescription: >\n  Manages servers\n  over SSH.\nother: x\n---\n# VPS\n")
	write(filepath.Join(dir, "commands", "ship.md"), "---\ndescription: \"Ship the branch\"\n---\nbody\n")
	write(filepath.Join(dir, "commands", "plain.md"), "no front matter\n")
	notes := map[string]string{}
	readNotes(filepath.Join(dir, "skills"), "", notes)
	readNotes(filepath.Join(dir, "commands"), "p:", notes)
	if notes["vps"] != "Manages servers over SSH." || notes["p:ship"] != "Ship the branch" {
		t.Fatalf("notes = %v", notes)
	}
	if _, ok := notes["p:plain"]; ok {
		t.Fatalf("plain got a note: %v", notes)
	}
}
