package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuideBlockKeepsTheUsersInstructions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GEMINI.md")
	os.WriteFile(path, []byte("# Mine\n\nUse tabs.\n"), 0o600)
	if err := keepBlock(path, "first"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	os.WriteFile(path, append(b, "\nAlso mine.\n"...), 0o600)
	if err := keepBlock(path, "second"); err != nil {
		t.Fatal(err)
	}
	if err := keepBlock(path, "second"); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	got := string(b)
	want := "# Mine\n\nUse tabs.\n\n" + blockStart + "\nsecond\n" + blockEnd + "\n\nAlso mine.\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want the user's 0600", fi.Mode().Perm())
	}
}

func TestAgentsReadTheGuideWhereItIsPlaced(t *testing.T) {
	home := t.TempDir()
	for _, p := range Presets {
		if len(GuideArgs(p.ID)) > 0 {
			continue
		}
		if len(guidePlaces[Base(p.ID)]) == 0 {
			if GuideNote(p.ID) == "" {
				t.Errorf("%s: no way to give it the guide", p.ID)
			}
			continue
		}
		if err := PlaceGuide(p.ID, home); err != nil {
			t.Errorf("%s: %v", p.ID, err)
		}
		for _, pl := range guidePlaces[Base(p.ID)] {
			b, err := os.ReadFile(filepath.Join(home, pl.path))
			if err != nil || !strings.Contains(string(b), "ENOWX_SESSION") {
				t.Errorf("%s: %s: %v", p.ID, pl.path, err)
			}
		}
	}
}
