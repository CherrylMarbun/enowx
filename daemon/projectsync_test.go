package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/core/transcript"
	"github.com/enowx/enowx/proto"
)

// A project copied to a server and worked on in both places comes back
// together: each side's files, commits, sessions and conversations go to
// the other, a file changed on both taking the newer.
func TestSyncProjectBothWays(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	// An agent that waits, rather than whichever claude the host has.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	gitIn := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=a"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(path, text string) {
		t.Helper()
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		data, _ := os.ReadFile(path)
		return string(data)
	}
	repo := filepath.Join(t.TempDir(), "app")
	write(filepath.Join(repo, "main.go"), "package main")
	write(filepath.Join(repo, "README.md"), "app")
	write(filepath.Join(repo, "gone.txt"), "to delete")
	gitIn(repo, "init", "-q", "-b", "main")
	gitIn(repo, "add", ".")
	gitIn(repo, "commit", "-qm", "first")
	gitIn(repo, "branch", "side")
	commitOn := func(dir, branch, msg string) {
		c := gitIn(dir, "commit-tree", branch+"^{tree}", "-p", branch, "-m", msg)
		gitIn(dir, "update-ref", "refs/heads/"+branch, c)
	}

	srcSrv, dstSrv := startServer(t), startServer(t)
	src, dst := connect(t, srcSrv, nil), connect(t, dstSrv, nil)
	if _, err := src.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := src.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "claude", Name: "agent", BranchMode: proto.BranchNone}); err != nil {
		t.Fatal(err)
	}
	wt, err := src.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "shell", Name: "feature", BranchMode: proto.BranchNew, Branch: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(wt.Dir, "wip.txt"), "started")
	time.Sleep(1100 * time.Millisecond) // the conversation is newer than its session
	line := func(dir, text string) string {
		return `{"type":"user","cwd":"` + dir + `","message":{"role":"user","content":"` + text + `"}}` + "\n"
	}
	srcConvo := filepath.Join(home, ".claude", "projects", transcript.ClaudeFolder(repo), "abc123.jsonl")
	write(srcConvo, line(repo, "hi"))

	// Copied to the other host, as Copy to another host does.
	info, err := src.PackTransfer(proto.TransferPackParams{Project: repo})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CopyTransfer(src, dst, info.ID, info.Size, nil); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "app")
	copied, err := dst.UnpackTransfer(info.ID, dest)
	if err != nil {
		t.Fatal(err)
	}
	scan := func() [2]proto.ProjectSyncState {
		t.Helper()
		a, err := src.ScanProjectSync(repo, false)
		if err != nil {
			t.Fatal(err)
		}
		b, err := dst.ScanProjectSync(dest, false)
		if err != nil {
			t.Fatal(err)
		}
		return [2]proto.ProjectSyncState{a, b}
	}
	st := scan()
	if p := projectsync.Compare(nil, st[0], st[1]); !p.Empty() {
		t.Fatalf("a fresh copy differs: %+v", p)
	}
	base := projectsync.BaseOf(st[0], st[1])

	// Work on the server: a commit, a new file, a file deleted, the
	// worktree's work, the conversation going on, a new session.
	write(filepath.Join(dest, "main.go"), "package main // on the server")
	gitIn(dest, "commit", "-qam", "on the server")
	write(filepath.Join(dest, "server.txt"), "new there")
	os.Remove(filepath.Join(dest, "gone.txt"))
	var dstWorktree proto.SessionInfo
	for _, s := range copied.Sessions {
		if s.Name == "feature" {
			dstWorktree = s
		}
	}
	write(filepath.Join(dstWorktree.Dir, "wip.txt"), "went on")
	dstConvo := filepath.Join(home, ".claude", "projects", transcript.ClaudeFolder(dest), "abc123.jsonl")
	write(dstConvo, read(dstConvo)+line(dest, "more"))
	if _, err := dst.CreateSession(proto.SessionCreateParams{Project: dest, Agent: "shell", Name: "later", BranchMode: proto.BranchNone}); err != nil {
		t.Fatal(err)
	}
	// Work here meanwhile, and README changed on both, the server's last;
	// the branch side committed to on both.
	commitOn(dest, "side", "there")
	commitOn(repo, "side", "here")
	write(filepath.Join(repo, "local.txt"), "new here")
	write(filepath.Join(repo, "README.md"), "app, here")
	time.Sleep(20 * time.Millisecond)
	write(filepath.Join(dest, "README.md"), "app, there")

	st = scan()
	plan := projectsync.Compare(&base, st[0], st[1])
	if plan.Conflicts() != 1 {
		t.Fatalf("conflicts in %+v", plan)
	}
	clients := [2]*client.Client{src, dst}
	res, next, err := projectsync.Run{
		Daemons: [2]projectsync.Daemon{src, dst}, Names: [2]string{"This PC", "Server"}, States: st, Plan: plan,
		Carry: func(from projectsync.Side, id string, size int64) error {
			return client.CopyTransfer(clients[from], clients[from.Other()], id, size, nil)
		},
	}.Do()
	if err != nil {
		t.Fatal(err)
	}

	for rel, want := range map[string]string{"main.go": "package main // on the server", "server.txt": "new there", "README.md": "app, there"} {
		if got := read(filepath.Join(repo, rel)); got != want {
			t.Errorf("%s here = %q, want %q", rel, got, want)
		}
	}
	if fileExists(filepath.Join(repo, "gone.txt")) {
		t.Error("the file deleted there is still here")
	}
	if got := read(filepath.Join(dest, "local.txt")); got != "new here" {
		t.Errorf("local.txt there = %q", got)
	}
	if gitIn(repo, "rev-parse", "main") != gitIn(dest, "rev-parse", "main") {
		t.Error("main didn't come here")
	}
	if gitIn(repo, "rev-parse", "side-from-server") != gitIn(dest, "rev-parse", "side") {
		t.Error("the branch that went its own way on both isn't kept beside")
	}
	if status := gitIn(repo, "status", "--porcelain"); strings.Contains(status, "main.go") {
		t.Errorf("main.go shows as changed here, the branch having moved: %q", status)
	}
	if got := read(filepath.Join(wt.Dir, "wip.txt")); got != "went on" {
		t.Errorf("the worktree's work here = %q", got)
	}
	if got := read(srcConvo); !strings.Contains(got, `"content":"more"`) || !strings.Contains(got, `"cwd":"`+repo+`"`) || strings.Contains(got, dest) {
		t.Errorf("the conversation here: %s", got)
	}
	names := map[string]bool{}
	sessions, _ := src.Sessions()
	for _, s := range sessions {
		names[s.Name] = true
	}
	if !names["later"] || len(sessions) != 3 {
		t.Errorf("sessions here: %+v (result %+v)", sessions, res[projectsync.Here])
	}

	// Nothing differs after.
	st = scan()
	if p := projectsync.Compare(&next, st[0], st[1]); len(p.Files) > 0 || len(p.Sessions) > 0 {
		t.Errorf("still differs: %+v", p)
	}
}
