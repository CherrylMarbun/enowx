package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestCreateIsIdempotent(t *testing.T) {
	repo := t.TempDir()
	run(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "add", ".")
	run(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init")

	root := t.TempDir()
	p := proto.WorktreeCreateParams{Repo: repo, Name: "agent-1"}
	first, err := Create(root, p)
	if err != nil {
		t.Fatal(err)
	}
	if first.Branch != "enowx/agent-1" {
		t.Fatalf("branch = %q", first.Branch)
	}
	if _, err := os.Stat(filepath.Join(first.Path, "a.txt")); err != nil {
		t.Fatalf("worktree not checked out: %v", err)
	}
	again, err := Create(root, p)
	if err != nil {
		t.Fatal(err)
	}
	// The same worktree, there already rather than made.
	if !first.Created || again.Created || again.Path != first.Path || again.Branch != first.Branch {
		t.Fatalf("second create = %+v, first %+v", again, first)
	}
}

func TestCreateRejectsBadNames(t *testing.T) {
	for _, name := range []string{"", "../x", "a b", "-x"} {
		if _, err := Create(t.TempDir(), proto.WorktreeCreateParams{Repo: ".", Name: name}); err == nil {
			t.Errorf("Create(%q) succeeded", name)
		}
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
