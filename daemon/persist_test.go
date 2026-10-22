package daemon

import (
	"os"
	"runtime"
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestSessionsSurviveADaemonRestart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	dir := t.TempDir()
	work := t.TempDir()

	first, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := connect(t, first, nil)
	shell, err := c.CreateSession(proto.SessionCreateParams{Agent: "shell", Dir: work, Cols: 80, Rows: 24, Env: []string{"PS1=$ "}})
	if err != nil {
		t.Fatal(err)
	}
	custom, err := c.CreateSession(proto.SessionCreateParams{Agent: "custom", Command: []string{"sh", "-c", "echo custom-output; sleep 60"}, Dir: work, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(shell.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Write([]byte("echo before-restart\r")); err != nil {
		t.Fatal(err)
	}
	watch(st).waitFor(t, "before-restart")
	cst, err := c.Attach(custom.ID)
	if err != nil {
		t.Fatal(err)
	}
	watch(cst).waitFor(t, "custom-output")
	first.Close() // what SIGTERM does: save, then stop

	second, err := NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)
	c2 := connect(t, second, nil)
	list, err := c2.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]proto.SessionInfo{}
	for _, s := range list {
		byID[s.ID] = s
	}
	if s := byID[shell.ID]; !s.Restored || s.Exited {
		t.Fatalf("shell after restart = %+v, want restored and running", s)
	}
	if s := byID[custom.ID]; !s.Restored || !s.Exited {
		t.Fatalf("custom command after restart = %+v, want restored but not rerun", s)
	}
	// The relaunched shell shows what it showed before, and runs again.
	st2, err := c2.Attach(shell.ID)
	if err != nil {
		t.Fatal(err)
	}
	w := watch(st2)
	w.waitFor(t, "before-restart")
	w.waitFor(t, "restarted this session")
	if _, err := st2.Write([]byte("echo after-restart\r")); err != nil {
		t.Fatal(err)
	}
	w.waitFor(t, "after-restart")
	cst2, err := c2.Attach(custom.ID)
	if err != nil {
		t.Fatal(err)
	}
	watch(cst2).waitFor(t, "custom-output")

	// Deleting forgets the session for the next daemon too.
	if err := c2.KillSession(custom.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(second.persist.historyFile(custom.ID)); !os.IsNotExist(err) {
		t.Fatalf("history file kept after delete: %v", err)
	}
	for _, r := range second.persist.saved() {
		if r.ID == custom.ID {
			t.Fatal("record kept after delete")
		}
	}
}
