package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// enowX's files move from the folder it shared with another app to its
// own, worktrees still git's and sessions pointing at their new place;
// the other app's files stay.
func TestMoveOldHomeTakesOnlyEnowXsFiles(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old, dir := filepath.Join(base, ".enowx"), filepath.Join(base, ".enowx-app")
	repo := filepath.Join(base, "repo")
	wt := filepath.Join(old, "worktrees", "001-ab", "feat")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", repo},
		{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"-C", repo, "worktree", "add", "-q", "-b", "feat", wt},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	files := map[string]string{
		"sessions.json":   `[{"id":"a","dir":"` + wt + `"}]`,
		"hosts.json":      "[]",
		"enowx.db":        "the other app's",
		"plugins/x":       "the other app's",
		"bin/cloudflared": "the other app's",
		"bin/enowxd":      "enowX's daemon",
	}
	for name, content := range files {
		p := filepath.Join(old, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// As on a server: the app installed its daemon in the new folder first.
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "enowxd"), []byte("the new daemon"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := moveHome(old, dir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "bin", "enowxd")); string(data) != "the new daemon" {
		t.Fatalf("the new daemon was replaced by %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".moving")); err == nil {
		t.Fatal("the move's lock stayed")
	}
	for _, name := range []string{"enowx.db", "plugins/x", "bin/cloudflared"} {
		if _, err := os.Stat(filepath.Join(old, name)); err != nil {
			t.Errorf("the other app's %s moved: %v", name, err)
		}
	}
	for _, name := range []string{"hosts.json", "worktrees/001-ab/feat/.git"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not moved: %v", name, err)
		}
	}
	newWT := filepath.Join(dir, "worktrees", "001-ab", "feat")
	if data, _ := os.ReadFile(filepath.Join(dir, "sessions.json")); !strings.Contains(string(data), `"dir":"`+newWT+`"`) {
		t.Fatalf("sessions.json: %s", data)
	}
	out, err := exec.Command("git", "-C", repo, "worktree", "list").CombinedOutput()
	if err != nil || !strings.Contains(string(out), newWT) {
		t.Fatalf("git's worktrees: %s %v", out, err)
	}
	if out, err := exec.Command("git", "-C", newWT, "status", "--short").CombinedOutput(); err != nil {
		t.Fatalf("the moved worktree: %s %v", out, err)
	}
	// Done once: a later start leaves both folders be.
	if err := moveHome(old, dir); err != nil {
		t.Fatal(err)
	}
}
