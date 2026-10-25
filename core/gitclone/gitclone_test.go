package gitclone

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddresses(t *testing.T) {
	for raw, name := range map[string]string{
		"https://github.com/enowx/enowx.git": "enowx",
		"git@github.com:user/my-app.git":     "my-app",
		"ssh://git@host:22/team/api":         "api",
		"https://gitlab.com/a/b/":            "b",
	} {
		if err := Valid(raw); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
		if got := Name(raw); got != name {
			t.Errorf("%s named %q, want %q", raw, got, name)
		}
	}
	for _, bad := range []string{"", "github.com/user/repo", "ftp://host/repo", "https://github.com"} {
		if Valid(bad) == nil {
			t.Errorf("%q taken", bad)
		}
	}
	if got := Redact("https://user:s3cret@github.com/a/b.git"); strings.Contains(got, "s3cret") {
		t.Errorf("redacted %q", got)
	}
	if got := Redact("https://ghp_token@github.com/a/b.git"); strings.Contains(got, "ghp_token") {
		t.Errorf("redacted %q", got)
	}
}

// git makes a repository with a commit in dir.
func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "first"}, {"branch", "feature"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestCloneReportsAndCleansUp(t *testing.T) {
	src := "file://" + repo(t)
	dest := filepath.Join(t.TempDir(), "work", "copy")
	var lines []string
	if err := Clone(context.Background(), Options{URL: src, Path: dest, Branch: "feature", Depth: 1}, func(p Progress) { lines = append(lines, p.Line) }); err != nil {
		t.Fatal(err)
	}
	if out, _ := exec.Command("git", "-C", dest, "branch", "--show-current").Output(); strings.TrimSpace(string(out)) != "feature" {
		t.Fatalf("cloned on %q", out)
	}
	if len(lines) == 0 {
		t.Error("no progress")
	}
	// Into a folder with something in it: refused, the folder kept.
	if err := Clone(context.Background(), Options{URL: src, Path: dest}, nil); err == nil || !strings.Contains(err.Error(), "isn't empty") {
		t.Fatalf("into a full folder: %v", err)
	}
	// A branch it hasn't: explained, nothing left behind.
	other := filepath.Join(t.TempDir(), "other")
	err := Clone(context.Background(), Options{URL: src, Path: other, Branch: "nope"}, nil)
	if err == nil || !strings.Contains(err.Error(), "no branch or tag") {
		t.Fatalf("missing branch: %v", err)
	}
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Fatal("a failed clone left its folder")
	}
	err = Clone(context.Background(), Options{URL: "file:///no/such/repo", Path: other}, nil)
	if err == nil || !strings.Contains(err.Error(), "no such repository") {
		t.Fatalf("missing repo: %v", err)
	}
}

// What git said is its error line, not the advice after it.
func TestExplainQuotesGitsError(t *testing.T) {
	said := "ERROR: Repository not found.\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.\n"
	err := explain(said, "git@github.com:a/b.git")
	if !strings.Contains(err.Error(), "git said: fatal: Could not read from remote repository.") {
		t.Errorf("explained as %q", err)
	}
}
