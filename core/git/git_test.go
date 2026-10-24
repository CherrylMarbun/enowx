package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A repository reached through a symlink keeps the path it was reached
// by, as macOS's /var is /private/var.
func TestTopLevelThroughASymlink(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	real := t.TempDir()
	repo := filepath.Join(real, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("no symlinks here")
	}
	for dir, want := range map[string]string{
		filepath.Join(link, "repo"):        filepath.Join(link, "repo"),
		filepath.Join(link, "repo", "sub"): filepath.Join(link, "repo"),
	} {
		got, err := TopLevel(dir)
		if err != nil || got != want {
			t.Errorf("TopLevel(%s) = %s, %v; want %s", dir, got, err, want)
		}
	}
}
