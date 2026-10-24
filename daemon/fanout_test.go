package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A fan-out starts one session per agent, each on its own branch and in
// its own worktree, all in one group and given the prompt.
func TestFanOutGivesOnePromptToSeveralAgents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	for _, name := range []string{"claude", "codex"} {
		script := "#!/bin/sh\necho \"" + name + " asked: $*\"\nsleep 30\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := gitRepo(t)
	srv := startServer(t)
	c := connect(t, srv, nil)
	list, err := c.FanOut(proto.FanOutParams{
		SessionCreateParams: proto.SessionCreateParams{Project: repo, BranchMode: proto.BranchNew, Branch: "fix-login", Prompt: "fix the login", Cols: 100, Rows: 10},
		Agents:              []string{"claude", "codex", "claude"},
	})
	if err != nil || len(list) != 3 {
		t.Fatalf("fan-out: %+v, %v", list, err)
	}
	want := []string{"fix-login-claude", "fix-login-codex", "fix-login-claude-2"}
	dirs := map[string]bool{}
	for i, s := range list {
		if s.Branch != want[i] || !s.Worktree || s.Group == "" || s.Group != list[0].Group {
			t.Fatalf("session %d: %+v", i, s)
		}
		dirs[s.Dir] = true
		sess, _ := srv.sessions.Get(s.ID)
		waitFor(t, s.Agent+" given the prompt", func() bool {
			return strings.Contains(sess.Terminal().Text(), "fix the login")
		})
	}
	if len(dirs) != 3 {
		t.Fatalf("worktrees shared: %v", dirs)
	}
	if _, err := c.FanOut(proto.FanOutParams{SessionCreateParams: proto.SessionCreateParams{Project: repo, BranchMode: proto.BranchNew, Branch: "x"}, Agents: []string{"claude"}}); err == nil {
		t.Fatal("a fan-out of one agent")
	}
}
