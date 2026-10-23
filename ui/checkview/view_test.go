package checkview

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

func TestChecksShowHowTheyWent(t *testing.T) {
	v := New()
	v.loaded = true
	v.info = proto.ChecksInfo{
		Checks: []proto.Check{{Name: "lint", Command: "make lint"}, {Name: "test", Command: "make test"}, {Name: "build", Command: "make build"}},
		Runs: []proto.CheckRun{
			{Name: "lint", State: proto.CheckPassed, Duration: 1200 * time.Millisecond, Output: "ok\n"},
			{Name: "test", State: proto.CheckFailed, ExitCode: 2, Duration: 3 * time.Second, Output: "--- FAIL: TestX\nFAIL\n"},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, Options{}) }, 520, 600)
	tt.Frame()
	for _, want := range []string{"found in the project's files", "Run all", "passed · 1.2s", "failed (exit 2) · 3s", "not run", "make test"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("--- FAIL: TestX\nFAIL") {
		t.Error("a folded check shows its output")
	}
	if err := tt.Click("test"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("--- FAIL: TestX\nFAIL") {
		t.Fatalf("unfolded: %q", tt.Texts())
	}

	// While one runs, Stop replaces Run all.
	v.info.Runs[1] = proto.CheckRun{Name: "test", State: proto.CheckRunning, Started: time.Now()}
	tt.Frame()
	if !tt.HasText("Stop") || tt.HasText("Run all") {
		t.Fatalf("running: %q", tt.Texts())
	}

	// Edit opens the project's checks to change.
	if err := tt.Click("Edit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Checks of this project") || !tt.HasText("Add a check") {
		t.Fatalf("edit: %q", tt.Texts())
	}
}

func TestLastLines(t *testing.T) {
	if got := lastLines("a\nb\nc\n", 2); got != "…\nb\nc" {
		t.Fatalf("%q", got)
	}
	if got := lastLines("FAIL\tpkg\n\x1b[31mred\x1b[0m", 5); got != "FAIL    pkg\nred" {
		t.Fatalf("%q", got)
	}
}
