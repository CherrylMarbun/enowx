package checks

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commands(checks []proto.Check) string {
	var out []string
	for _, c := range checks {
		out = append(out, c.Name+"="+c.Command)
	}
	return strings.Join(out, ", ")
}

func TestDetect(t *testing.T) {
	goDir := t.TempDir()
	write(t, goDir, "go.mod", "module x\n")
	if got := commands(Detect(goDir)); got != "vet=go vet ./..., build=go build ./..., test=go test ./..." {
		t.Errorf("go: %s", got)
	}
	// A Makefile's targets come before the language's commands.
	write(t, goDir, "Makefile", "build:\n\tgo build\ntest: build\n\tgo test\nlint:\n\tgo vet\nVAR := x\n")
	if got := commands(Detect(goDir)); got != "lint=make lint, build=make build, test=make test" {
		t.Errorf("makefile: %s", got)
	}
	node := t.TempDir()
	write(t, node, "package.json", `{"scripts":{"test":"vitest","type-check":"tsc","dev":"vite"}}`)
	write(t, node, "pnpm-lock.yaml", "")
	if got := commands(Detect(node)); got != "typecheck=pnpm run type-check, test=pnpm run test" {
		t.Errorf("node: %s", got)
	}
	if got := Detect(t.TempDir()); len(got) != 0 {
		t.Errorf("empty folder: %+v", got)
	}
}

func TestStoreOverridesPerProject(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module x\n")
	path := filepath.Join(t.TempDir(), "checks.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, custom := s.For("/p", dir); custom {
		t.Fatal("custom before any was set")
	}
	if err := s.Set("/p", []proto.Check{{Command: "  just ci "}, {Name: "x", Command: ""}}); err != nil {
		t.Fatal(err)
	}
	again, _ := OpenStore(path)
	if got, custom := again.For("/p", dir); !custom || commands(got) != "just=just ci" {
		t.Fatalf("after setting: %v %s", custom, commands(got))
	}
	if err := again.Set("/p", nil); err != nil {
		t.Fatal(err)
	}
	if got, custom := again.For("/p", dir); custom || len(got) != 3 {
		t.Fatalf("after clearing: %v %s", custom, commands(got))
	}
}

func waitDone(t *testing.T, r *Runner, session string) []proto.CheckRun {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		runs := r.Results(session)
		done := len(runs) > 0
		for _, run := range runs {
			if run.State == proto.CheckQueued || run.State == proto.CheckRunning {
				done = false
			}
		}
		if done {
			return runs
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("checks never ended: %+v", r.Results(session))
	return nil
}

func TestRunnerRunsInOrderAndKeepsResults(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	var r Runner
	dir := t.TempDir()
	r.Run("s", dir, []proto.Check{
		{Name: "ok", Command: "echo fine; echo $CI"},
		{Name: "bad", Command: "echo broken >&2; exit 3"},
	})
	runs := waitDone(t, &r, "s")
	if runs[0].State != proto.CheckPassed || runs[0].Output != "fine\ntrue\n" {
		t.Fatalf("ok = %+v", runs[0])
	}
	if runs[1].State != proto.CheckFailed || runs[1].ExitCode != 3 || runs[1].Output != "broken\n" {
		t.Fatalf("bad = %+v", runs[1])
	}
}

func TestStopEndsTheRunAndWhatItStarted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	var r Runner
	marker := filepath.Join(t.TempDir(), "survived")
	r.Run("s", t.TempDir(), []proto.Check{
		{Name: "slow", Command: "(sleep 2; touch " + marker + ") & sleep 30"},
		{Name: "next", Command: "true"},
	})
	time.Sleep(300 * time.Millisecond)
	r.Stop("s")
	runs := waitDone(t, &r, "s")
	if runs[0].State != proto.CheckStopped || runs[1].State != proto.CheckStopped {
		t.Fatalf("after stop: %+v", runs)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a process the check started outlived it")
	}
}
