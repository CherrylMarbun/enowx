package daemon

import (
	"runtime"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Ctrl+C that kills an agent leaves a shell to type into, in the same session.
func TestCtrlCLeavesAShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("SHELL", "/bin/sh")
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, err := c.CreateSession(proto.SessionCreateParams{
		Command: thenShell([]string{"sh", "-c", "echo agent-up; exec sleep 30"}), Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := watch(st)
	s.waitFor(t, "agent-up")
	if _, err := st.Write([]byte{0x03}); err != nil {
		t.Fatal(err)
	}
	s.waitFor(t, "$ ") // the shell's prompt: typing sooner may be dropped
	if _, err := st.Write([]byte("echo back-$((40+2))\r")); err != nil {
		t.Fatal(err)
	}
	s.waitFor(t, "back-42")
}
