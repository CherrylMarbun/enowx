//go:build !windows

package daemon

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/proto"
)

// Deleting a session stops what it started outside its terminal too, as a
// dev server an agent left running in tmux.
func TestDeletedSessionLeavesNothingRunning(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads processes' environment, which Linux shows")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	pidFile := t.TempDir() + "/escaped.pid"
	// setsid leaves the session's process group and terminal, as tmux does.
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c",
		"setsid sh -c 'echo $$ > " + pidFile + "; exec sleep 300' & sleep 300"}})
	if err != nil {
		t.Fatal(err)
	}
	pid := waitPID(t, pidFile)
	if err := c.KillSession(info.ID); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(reapGrace + 3*time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if !alive(pid) {
			return
		}
	}
	t.Fatalf("process %d outlived its deleted session", pid)
}

// Stopping a session's display, as closing its app's window does, stops
// what was run for its app (as `enowx display` marks it) and leaves the
// session's own program running.
func TestStoppedDisplayTakesItsAppNotTheSession(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads processes' environment, which Linux shows")
	}
	if ok, why := display.Available(); !ok {
		t.Skip(why)
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	pidFile := t.TempDir() + "/app.pid"
	info, err := c.CreateSession(proto.SessionCreateParams{Display: true, Command: []string{"sh", "-c",
		"ENOWX_DISPLAY=$DISPLAY setsid sh -c 'echo $$ > " + pidFile + "; exec sleep 300' & exec sleep 300"}})
	if err != nil {
		t.Fatal(err)
	}
	app := waitPID(t, pidFile)
	if err := c.StopDisplay(info.ID, info.Display); err != nil {
		t.Fatal(err)
	}
	gone := false
	for deadline := time.Now().Add(reapGrace + 3*time.Second); time.Now().Before(deadline) && !gone; time.Sleep(100 * time.Millisecond) {
		gone = !alive(app)
	}
	if !gone {
		t.Fatalf("the app (%d) outlived its display", app)
	}
	list, err := c.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.ID == info.ID && (s.Exited || s.Display != 0) {
			t.Fatalf("session after its display stopped: %+v", s)
		}
	}
}

// waitPID reads the pid a process wrote to file.
func waitPID(t *testing.T, file string) int {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		data, _ := os.ReadFile(file)
		if pid, _ := strconv.Atoi(strings.TrimSpace(string(data))); pid > 0 {
			return pid
		}
	}
	t.Fatal("the process never started")
	return 0
}

// alive reports whether pid runs: neither gone nor dead and waiting for a
// parent that doesn't reap it (a zombie).
func alive(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	i := strings.LastIndexByte(string(stat), ')')
	return i < 0 || len(stat) < i+3 || stat[i+2] != 'Z'
}

// An agent session's program runs in two wrappers (the session's, and
// thenShell's), with the display in its environment: stopping the display
// spares the three and what the agent runs for itself, its app aside.
func TestStoppedDisplaySparesTheAgent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads processes' environment, which Linux shows")
	}
	if ok, why := display.Available(); !ok {
		t.Skip(why)
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	agentPID, appPID := dir+"/agent.pid", dir+"/app.pid"
	info, err := c.CreateSession(proto.SessionCreateParams{Display: true, Command: thenShell([]string{"sh", "-c",
		"echo $$ > " + agentPID + "; ENOWX_DISPLAY=$DISPLAY setsid sh -c 'echo $$ > " + appPID + "; exec sleep 300' & sleep 300; true"})})
	if err != nil {
		t.Fatal(err)
	}
	agent, app := waitPID(t, agentPID), waitPID(t, appPID)
	if err := c.StopDisplay(info.ID, info.Display); err != nil {
		t.Fatal(err)
	}
	time.Sleep(reapGrace + time.Second)
	if alive(app) {
		t.Errorf("the app (%d) outlived its display", app)
	}
	if !alive(agent) {
		t.Errorf("the agent (%d) was stopped with the display", agent)
	}
}
