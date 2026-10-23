package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func TestReviewAWorktreeSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", writeGitConfig(t))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	srv := startServer(t)
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	if _, err := c.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{
		Project: repo, Agent: "custom", Command: []string{"sh", "-c", "sleep 30"},
		BranchMode: proto.BranchNew, Branch: "feat/x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info.Dir, "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := c.ReviewStatus(info.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Base != "main" || st.Branch != "feat/x" || st.Uncommitted != 1 || !st.CanMerge {
		t.Fatalf("status = %+v", st)
	}
	d, err := c.Diff(info.ID, "")
	if err != nil || len(d.Files) != 1 || d.Files[0].Path != "x.txt" {
		t.Fatalf("diff = %+v, %v", d, err)
	}
	if st, err = c.Commit(info.ID, "Add x"); err != nil || st.Ahead != 1 {
		t.Fatalf("after commit: %+v, %v", st, err)
	}
	if _, err := c.Merge(info.ID, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "x.txt")); err != nil {
		t.Fatal("not merged into the project folder")
	}

	// Discarding the worktree ends its session.
	if err := c.Discard(info.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(info.Dir); !os.IsNotExist(err) {
		t.Fatal("worktree still there")
	}
	if list, _ := c.Sessions(); len(list) != 0 {
		t.Fatalf("sessions = %+v", list)
	}
}

func TestSlowRequestsDontHoldOthersUp(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	release := make(chan struct{})
	slowMethods["test.slow"] = true
	handlers["test.slow"] = func(*conn, json.RawMessage) (any, error) { <-release; return ok, nil }
	t.Cleanup(func() {
		delete(slowMethods, "test.slow")
		delete(handlers, "test.slow")
	})
	done := make(chan error, 1)
	go func() { done <- c.Call("test.slow", nil, nil) }()
	time.Sleep(50 * time.Millisecond) // the slow request goes first
	listed := make(chan error, 1)
	go func() {
		_, err := c.Sessions()
		listed <- err
	}()
	select {
	case err := <-listed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a request waited for a slow one")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func writeGitConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(path, []byte("[user]\n\tname = t\n\temail = t@t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConflictsBetweenSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", writeGitConfig(t))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	srv := startServer(t)
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	if _, err := c.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	start := func(branch string) proto.SessionInfo {
		info, err := c.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "custom", Command: []string{"sh", "-c", "sleep 30"}, BranchMode: proto.BranchNew, Branch: branch})
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	a, b := start("feat/a"), start("feat/b")
	for _, s := range []proto.SessionInfo{a, b} {
		if err := os.WriteFile(filepath.Join(s.Dir, "shared.go"), []byte("package x // "+s.Branch+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a.Dir, "only-a.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := c.Conflicts(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].File != "shared.go" || !got[0].Lines || len(got[0].Sessions) != 2 {
		t.Fatalf("conflicts = %+v", got)
	}
}

// A project's open issues come from gh on its host.
func TestProjectIssuesComeFromGh(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	gh := "#!/bin/sh\necho \"$*\" > \"$(dirname \"$0\")/args\"\n" +
		`echo '[{"number":12,"title":"Login form forgets the email","body":"Steps: ...","url":"https://github.com/o/r/issues/12","labels":[{"name":"bug"}],"author":{"login":"ada"}}]'` + "\n"
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	c := connect(t, startServer(t), nil)
	repo := gitRepo(t)
	issues, err := c.Issues(repo, "is:issue login")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Number != 12 || issues[0].Labels[0] != "bug" || issues[0].Author != "ada" {
		t.Fatalf("issues %+v", issues)
	}
	args, _ := os.ReadFile(filepath.Join(bin, "args"))
	if !strings.Contains(string(args), "issue list --state open") || !strings.Contains(string(args), "--search is:issue login") {
		t.Fatalf("gh %s", args)
	}
}
