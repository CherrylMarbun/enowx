package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// isolate keeps the user's git configuration (signing, hooks) out of the
// tests and gives commits an author.
func isolate(t *testing.T) {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n\tname = t\n\temail = t@t\n[init]\n\tdefaultBranch = main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// project makes a repository on main with one commit, and a worktree of
// it under root on branch feature.
func project(t *testing.T) (repo, root string, w Where) {
	t.Helper()
	isolate(t)
	repo, root = t.TempDir(), t.TempDir()
	gitIn(t, repo, "init", "-q")
	write(t, filepath.Join(repo, "a.txt"), "one\ntwo\nthree\n")
	write(t, filepath.Join(repo, "gone.txt"), "bye\n")
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", "init")
	dir := filepath.Join(root, "feature")
	gitIn(t, repo, "worktree", "add", "-q", "-b", "feature", dir)
	return repo, root, Where{Dir: dir, Project: repo, Worktree: true}
}

func TestDiffShowsCommittedUncommittedAndUntracked(t *testing.T) {
	repo, _, w := project(t)
	write(t, filepath.Join(w.Dir, "a.txt"), "one\n2\nthree\n")
	gitIn(t, w.Dir, "commit", "-qam", "change a")
	gitIn(t, w.Dir, "rm", "-q", "gone.txt")
	write(t, filepath.Join(w.Dir, "new.txt"), "hello\nworld\n")
	// What main does after the branch started isn't the branch's doing.
	write(t, filepath.Join(repo, "main.txt"), "main\n")
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", "main moves on")

	base := DefaultBase(w)
	if base != "main" {
		t.Fatalf("default base = %q", base)
	}
	d, err := Diff(w.Dir, base)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]proto.DiffFile{}
	for _, f := range d.Files {
		got[f.Path] = f
	}
	if len(got) != 3 {
		t.Fatalf("files = %+v", d.Files)
	}
	a := got["a.txt"]
	if a.Status != proto.FileModified || a.Added != 1 || a.Removed != 1 || len(a.Hunks) != 1 {
		t.Fatalf("a.txt = %+v", a)
	}
	var plus proto.DiffLine
	for _, l := range a.Hunks[0].Lines {
		if l.Kind == '+' {
			plus = l
		}
	}
	if plus.Text != "2" || plus.New != 2 {
		t.Fatalf("added line = %+v", plus)
	}
	if f := got["gone.txt"]; f.Status != proto.FileDeleted || f.Removed != 1 {
		t.Fatalf("gone.txt = %+v", f)
	}
	if f := got["new.txt"]; f.Status != proto.FileAdded || f.Added != 2 {
		t.Fatalf("new.txt = %+v", f)
	}
	if _, ok := got["main.txt"]; ok {
		t.Fatal("main's own commit shows as the branch's")
	}
}

func TestCommitMergeAndDiscard(t *testing.T) {
	repo, root, w := project(t)
	write(t, filepath.Join(w.Dir, "b.txt"), "b\n")
	st, err := Status(w, "main")
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "feature" || st.Uncommitted != 1 || !st.CanMerge {
		t.Fatalf("status = %+v", st)
	}
	if err := Merge(w, "main"); err == nil {
		t.Fatal("merged uncommitted changes")
	}
	if err := Commit(w.Dir, "add b"); err != nil {
		t.Fatal(err)
	}
	if err := Commit(w.Dir, "again"); err == nil {
		t.Fatal("committed nothing")
	}
	if st, _ := Status(w, "main"); st.Ahead != 1 || st.Uncommitted != 0 {
		t.Fatalf("after commit: %+v", st)
	}

	// A project folder with changes of its own isn't merged into.
	write(t, filepath.Join(repo, "a.txt"), "dirty\n")
	if err := Merge(w, "main"); err == nil || !strings.Contains(err.Error(), "uncommitted") {
		t.Fatalf("merge into a dirty folder: %v", err)
	}
	gitIn(t, repo, "checkout", "-q", "--", "a.txt")
	if err := Merge(w, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "b.txt")); err != nil {
		t.Fatal("the merge didn't bring b.txt")
	}

	if err := Discard(t.TempDir(), w, true); err == nil {
		t.Fatal("discarded a worktree outside enowX's folder")
	}
	if err := Discard(root, w, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.Dir); !os.IsNotExist(err) {
		t.Fatal("the worktree is still there")
	}
	if out := gitIn(t, repo, "branch", "--list", "feature"); out != "" {
		t.Fatalf("branch still there: %q", out)
	}
}

func TestMergeConflictLeavesTheProjectAsItWas(t *testing.T) {
	repo, _, w := project(t)
	write(t, filepath.Join(w.Dir, "a.txt"), "feature\n")
	gitIn(t, w.Dir, "commit", "-qam", "feature edit")
	write(t, filepath.Join(repo, "a.txt"), "main\n")
	gitIn(t, repo, "commit", "-qam", "main edit")
	if err := Merge(w, "main"); err == nil {
		t.Fatal("a conflicting merge succeeded")
	}
	if out := gitIn(t, repo, "status", "--porcelain"); out != "" {
		t.Fatalf("project folder left as %q", out)
	}
}

func TestPushSetsTheUpstream(t *testing.T) {
	_, _, w := project(t)
	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare")
	gitIn(t, w.Dir, "remote", "add", "origin", remote)
	write(t, filepath.Join(w.Dir, "c.txt"), "c\n")
	if err := Commit(w.Dir, "add c"); err != nil {
		t.Fatal(err)
	}
	if st, _ := Status(w, "main"); st.Upstream != "" || st.Remote != "origin" {
		t.Fatalf("before push: %+v", st)
	}
	if err := Push(w.Dir); err != nil {
		t.Fatal(err)
	}
	st, _ := Status(w, "main")
	if st.Upstream != "origin/feature" || st.Unpushed != 0 {
		t.Fatalf("after push: %+v", st)
	}
}
