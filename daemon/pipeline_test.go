package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A pipeline runs its stages one after another on one branch, each
// starting from what the one before ended with.
func TestPipelineHandsEachStageTheLastOnesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	// A "claude" that says what it was asked; the planner also says its
	// plan. Its hooks are played by the test.
	script := "#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\necho \"asked: $last\"\ncase \"$last\" in *planning*) echo PLAN-MARKER-42;; esac\nsleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	srv := startServer(t)
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	stages := proto.DefaultPipeline("claude")[:2]
	pl, err := c.StartPipeline(proto.PipelineStartParams{Project: repo, Task: "add a search box", Branch: "feat/search", Stages: stages})
	if err != nil {
		t.Fatal(err)
	}
	stage := func(i int) proto.PipelineStageRun {
		list, err := c.Pipelines()
		if err != nil || len(list) != 1 {
			t.Fatalf("pipelines %+v, %v", list, err)
		}
		return list[0].Stages[i]
	}
	screen := func(id string) string {
		text, _ := c.Screen(id)
		return text
	}
	done := func(id string) {
		if err := c.Call(proto.MethodSessionState, proto.SessionStateParams{ID: id, State: proto.StateDone}, nil); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "the planner to plan", func() bool {
		return stage(0).Session != "" && strings.Contains(screen(stage(0).Session), "PLAN-MARKER-42")
	})
	if stage(1).Status != proto.PipelinePending {
		t.Fatalf("the second stage started early: %+v", stage(1))
	}
	done(stage(0).Session)
	waitFor(t, "the implementer to start", func() bool { return stage(1).Session != "" })
	waitFor(t, "the plan handed on", func() bool { return strings.Contains(screen(stage(1).Session), "PLAN-MARKER-42") })
	sessions, _ := c.Sessions()
	var dirs []string
	for _, s := range sessions {
		if s.Branch != "feat/search" || !s.Worktree {
			t.Fatalf("session %s on %q, worktree %v", s.ID, s.Branch, s.Worktree)
		}
		dirs = append(dirs, s.Dir)
	}
	if len(dirs) != 2 || dirs[0] != dirs[1] {
		t.Fatalf("the stages ran in %q", dirs)
	}
	done(stage(1).Session)
	waitFor(t, "the pipeline to finish", func() bool {
		list, _ := c.Pipelines()
		return len(list) == 1 && list[0].Status == proto.PipelineDone
	})
	if out := stage(0).Output; !strings.Contains(out, "PLAN-MARKER-42") {
		t.Fatalf("the plan's output: %q", out)
	}
	if err := c.RemovePipeline(pl.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := c.Pipelines(); len(list) != 0 {
		t.Fatalf("removed, still listed: %+v", list)
	}
}

func TestPipelineCanBeCancelledAndChecksItsStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	fakeAgent(t)
	srv := startServer(t)
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	for _, bad := range []proto.PipelineStartParams{
		{Project: repo, Task: "", Branch: "x", Stages: proto.DefaultPipeline("claude")},
		{Project: repo, Task: "t", Branch: "fix/old-bug", Stages: proto.DefaultPipeline("claude")}, // exists
		{Project: repo, Task: "t", Branch: "y", Stages: proto.DefaultPipeline("nope")},
		{Project: t.TempDir(), Task: "t", Branch: "z", Stages: proto.DefaultPipeline("claude")}, // no git
	} {
		if _, err := c.StartPipeline(bad); err == nil {
			t.Errorf("started %+v", bad)
		}
	}
	pl, err := c.StartPipeline(proto.PipelineStartParams{Project: repo, Task: "t", Branch: "feat/c", Stages: proto.DefaultPipeline("claude")})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the first stage", func() bool {
		list, _ := c.Pipelines()
		return len(list) == 1 && list[0].Stages[0].Session != ""
	})
	if err := c.CancelPipeline(pl.ID); err != nil {
		t.Fatal(err)
	}
	list, _ := c.Pipelines()
	if list[0].Status != proto.PipelineCancelled || list[0].Stages[2].Status != proto.PipelineCancelled {
		t.Fatalf("cancelled: %+v", list[0])
	}
}

// A daemon started again goes on with the pipelines the last one ran.
func TestPipelineGoesOnAfterARestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	fakeAgent(t)
	dir := t.TempDir()
	srv, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	if _, err := c.StartPipeline(proto.PipelineStartParams{Project: repo, Task: "t", Branch: "feat/r", Stages: proto.DefaultPipeline("claude")[:2]}); err != nil {
		t.Fatal(err)
	}
	var first string
	waitFor(t, "the first stage", func() bool {
		list, _ := c.Pipelines()
		first = list[0].Stages[0].Session
		return first != ""
	})
	srv.Close()

	again, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	c2 := connect(t, again, nil)
	if err := c2.Call(proto.MethodSessionState, proto.SessionStateParams{ID: first, State: proto.StateDone}, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the second stage after the restart", func() bool {
		list, _ := c2.Pipelines()
		return len(list) == 1 && list[0].Stages[1].Session != ""
	})
}
