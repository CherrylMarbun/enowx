package daemon

import (
	"os"
	"runtime"
	"testing"

	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
)

func TestHookReportsTheAgentsState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	// A short folder: Unix socket paths are limited to about 100 bytes.
	home, err := os.MkdirTemp("", "enx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("ENOWX_HOME", home)
	path, err := transport.SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := startServer(t)
	go srv.Serve(l)
	t.Cleanup(func() { l.Close() })

	sess, err := srv.sessions.Create(session.Spec{Command: []string{"sh", "-c", "cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(session.EnvSession, sess.ID())
	for _, state := range []string{proto.StateWorking, proto.StateWaiting} {
		Hook([]string{state})
		if got := sess.Info().State; got != state {
			t.Fatalf("after hook %s: state %q", state, got)
		}
	}
	// Answering the agent ends its wait.
	srv.sessions.Input(sess, []byte("1"))
	if got := sess.Info().State; got != "" {
		t.Fatalf("after answering: state %q", got)
	}
	// After a turn, the next prompt ends done; typing it doesn't.
	Hook([]string{proto.StateDone})
	srv.sessions.Input(sess, []byte("next"))
	if got := sess.Info().State; got != proto.StateDone {
		t.Fatalf("while typing a prompt: state %q", got)
	}
	srv.sessions.Input(sess, []byte("\r"))
	if got := sess.Info().State; got != "" {
		t.Fatalf("after sending it: state %q", got)
	}
	// An unknown state is refused, and the session keeps its own.
	Hook([]string{"bogus"})
	if got := sess.Info().State; got != "" {
		t.Fatalf("after a bogus state: %q", got)
	}
}
