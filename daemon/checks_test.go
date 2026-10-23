package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func TestChecksOfASession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	events := make(chan string, 64)
	c := connect(t, srv, func(ev proto.Event) {
		var ref proto.SessionRef
		if ev.Name == proto.EventChecksChanged && json.Unmarshal(ev.Data, &ref) == nil {
			events <- ref.ID
		}
	})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{Project: dir, Agent: "custom", Command: []string{"sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Checks(info.ID)
	if err != nil || len(got.Checks) != 3 || got.Custom {
		t.Fatalf("found = %+v, %v", got, err)
	}
	got, err = c.SetChecks(info.ID, []proto.Check{{Name: "pass", Command: "echo ok"}, {Name: "fail", Command: "exit 1"}})
	if err != nil || !got.Custom || len(got.Checks) != 2 {
		t.Fatalf("set = %+v, %v", got, err)
	}
	if _, err := c.RunChecks(info.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-events:
		case <-deadline:
			t.Fatal("checks never ended")
		}
		got, _ = c.Checks(info.ID)
		if len(got.Runs) == 2 && got.Runs[0].State == proto.CheckPassed && got.Runs[1].State == proto.CheckFailed {
			return
		}
	}
}
