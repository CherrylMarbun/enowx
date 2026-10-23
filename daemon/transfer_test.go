package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/transcript"
	"github.com/enowx/enowx/proto"
)

// A project copied to another host comes with what git keeps (not its
// dependencies), and its sessions start again there: a worktree's
// uncommitted work with it, Claude Code on its own conversation.
func TestCopyProjectToAnotherHost(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := filepath.Join(t.TempDir(), "app")
	for rel, text := range map[string]string{"main.go": "package main", ".gitignore": "node_modules/\n.env\n", "node_modules/x.js": "x", ".env": "SECRET=1"} {
		p := filepath.Join(repo, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-qm", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}

	src, dst := startServer(t), startServer(t)
	from, to := connect(t, src, nil), connect(t, dst, nil)
	if _, err := from.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := from.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "shell", Name: "shell", BranchMode: proto.BranchNone}); err != nil {
		t.Fatal(err)
	}
	wt, err := from.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "shell", Name: "feature", BranchMode: proto.BranchNew, Branch: "feat"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt.Dir, "wip.txt"), []byte("not committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := from.CreateSession(proto.SessionCreateParams{Project: repo, Agent: "claude", Name: "agent", BranchMode: proto.BranchNone}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // the conversation is newer than its session
	convo := filepath.Join(home, ".claude", "projects", transcript.ClaudeFolder(repo), "abc123.jsonl")
	_ = os.MkdirAll(filepath.Dir(convo), 0o700)
	line := `{"type":"user","cwd":"` + repo + `","message":{"role":"user","content":"hi"}}` + "\n"
	if err := os.WriteFile(convo, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := from.PackTransfer(proto.TransferPackParams{Project: repo})
	if err != nil {
		t.Fatal(err)
	}
	if info.Sessions != 3 {
		t.Fatalf("packed %+v", info)
	}
	if err := client.CopyTransfer(from, to, info.ID, info.Size, nil); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "copy")
	res, err := to.UnpackTransfer(info.ID, dest)
	if err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{"main.go", ".gitignore", ".git/HEAD"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("%s didn't come: %v", rel, err)
		}
	}
	for _, rel := range []string{"node_modules", ".env"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err == nil {
			t.Errorf("%s came along", rel)
		}
	}
	if res.Project.Path != dest || len(res.Sessions) != 3 {
		t.Fatalf("result %+v", res)
	}
	byName := map[string]proto.SessionInfo{}
	for _, s := range res.Sessions {
		byName[s.Name] = s
	}
	if s := byName["shell"]; s.Dir != dest || s.Exited {
		t.Errorf("shell session %+v", s)
	}
	if s := byName["feature"]; !s.Worktree || s.Branch != "feat" {
		t.Errorf("worktree session %+v", s)
	} else if data, err := os.ReadFile(filepath.Join(s.Dir, "wip.txt")); err != nil || string(data) != "not committed" {
		t.Errorf("the worktree's work: %q, %v", data, err)
	}
	if s := byName["agent"]; !slices.Contains(s.Command, "--resume") || !slices.Contains(s.Command, "abc123") {
		t.Errorf("agent started with %q", s.Command)
	}
	placed, err := os.ReadFile(filepath.Join(home, ".claude", "projects", transcript.ClaudeFolder(dest), "abc123.jsonl"))
	if err != nil || !strings.Contains(string(placed), `"cwd":"`+dest+`"`) {
		t.Errorf("conversation placed: %s, %v", placed, err)
	}
	// The archive went with the unpacking, on the target.
	if file, _ := dst.transferFile(info.ID); fileExists(file) {
		t.Error("the archive stayed on the target")
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
