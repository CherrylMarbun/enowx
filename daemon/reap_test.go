package daemon

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

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
	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0 && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		data, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	if pid == 0 {
		t.Fatal("the escaped process never started")
	}
	if err := c.KillSession(info.ID); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(reapGrace + 3*time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
	}
	t.Fatalf("process %d outlived its deleted session", pid)
}
