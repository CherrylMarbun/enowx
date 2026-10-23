package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// An automation saved for a project runs its prompt when due, on a new
// branch when asked, records the run, and comes back after a restart.
func TestAutomationsRunWhenDue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nfor a in \"$@\"; do last=\"$a\"; done\necho \"asked: $last\"\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	srv, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := connect(t, srv, nil)
	repo := gitRepo(t)
	for _, bad := range []proto.Automation{
		{Name: "x", Project: repo, Agent: "claude", Prompt: "p", Schedule: "sometimes"},
		{Name: "", Project: repo, Agent: "claude", Prompt: "p", Schedule: "daily 09:00"},
		{Name: "x", Project: repo, Agent: "nope", Prompt: "p", Schedule: "daily 09:00"},
	} {
		if _, err := c.SaveAutomation(bad); err == nil {
			t.Errorf("saved %+v", bad)
		}
	}
	a, err := c.SaveAutomation(proto.Automation{Name: "Morning triage", Project: repo, Agent: "claude",
		Prompt: "Triage the new issues", Schedule: "Weekdays 09:00", Branch: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == "" || a.Schedule != "weekdays 09:00" || a.NextRun.IsZero() || a.NextRun.Weekday() == time.Saturday || a.NextRun.Weekday() == time.Sunday {
		t.Fatalf("saved %+v", a)
	}
	// Due: as the runner finds it at its next look.
	srv.startDueAutomations(a.NextRun.Add(time.Second))
	list, _ := c.Automations()
	if len(list) != 1 || list[0].LastSession == "" || list[0].LastError != "" || !list[0].NextRun.After(a.NextRun) {
		t.Fatalf("after running: %+v", list)
	}
	sessions, _ := c.Sessions()
	if len(sessions) != 1 || !strings.HasPrefix(sessions[0].Branch, "automation/morning-triage-") || !sessions[0].Worktree {
		t.Fatalf("sessions %+v", sessions)
	}
	waitFor(t, "the prompt", func() bool {
		text, _ := c.Screen(sessions[0].ID)
		return strings.Contains(text, "asked: Triage the new issues")
	})
	// Not due again until its next time; run now runs regardless.
	srv.startDueAutomations(list[0].NextRun.Add(-time.Minute))
	if s, _ := c.Sessions(); len(s) != 1 {
		t.Fatalf("ran before it was due: %d sessions", len(s))
	}
	if _, err := c.RunAutomation(a.ID); err != nil {
		t.Fatal(err)
	}
	srv.Close()
	again, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	c2 := connect(t, again, nil)
	if list, _ := c2.Automations(); len(list) != 1 || list[0].Name != "Morning triage" || list[0].LastRun.IsZero() {
		t.Fatalf("after a restart: %+v", list)
	}
	if err := c2.RemoveAutomation(a.ID); err != nil {
		t.Fatal(err)
	}
}
