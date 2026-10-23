package daemon

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
)

// hookTimeout bounds a hook: the agent waits for it.
const hookTimeout = 3 * time.Second

// Hook reports the state of the agent in this process's session to the
// daemon: `enowx hook <state>` (or enowxd's), which the agent's hooks run
// (agent.HookArgs). It prints nothing, since agents read what hooks print,
// and never fails the hook: a state that can't be reported is dropped.
func Hook(args []string) {
	id := os.Getenv(session.EnvSession)
	if len(args) != 1 || id == "" {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, err := dialDaemon()
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.Call(proto.MethodSessionState, proto.SessionStateParams{ID: id, State: args[0]}, nil)
	}()
	select {
	case <-done:
	case <-time.After(hookTimeout):
	}
}

// Display gives this process's session a virtual display and prints the
// shell command that sends a program's windows there, drawn sharp on the
// user's screen: `eval "$("$ENOWX_BIN" display)"`, which an agent runs
// before starting a desktop app (see agent.Guide).
func Display() error {
	id := os.Getenv(session.EnvSession)
	if id == "" {
		return errors.New("not in an enowX session")
	}
	c, err := dialDaemon()
	if err != nil {
		return err
	}
	defer c.Close()
	var d proto.DisplayInfo
	if err := c.Call(proto.MethodSessionDisplay, proto.SessionRef{ID: id}, &d); err != nil {
		return err
	}
	env := d.Env
	if len(env) == 0 {
		env = []string{"DISPLAY=" + d.Display} // an older daemon
	}
	fmt.Println(exportLine(env))
	fmt.Fprintf(os.Stderr, "Windows opened on %s show in enowX's Browser tool (port %d, display).\n", d.Display, d.Port)
	return nil
}

// dialDaemon connects to the daemon of this machine.
func dialDaemon() (*client.Client, error) {
	path, err := transport.SocketPath()
	if err != nil {
		return nil, err
	}
	nc, err := transport.DialLocal(path, nil)
	if err != nil {
		return nil, err
	}
	c, err := client.New(nc, nil)
	if err != nil {
		nc.Close()
		return nil, err
	}
	return c, nil
}

// exportLine is a shell command exporting env (NAME=value entries).
func exportLine(env []string) string {
	var b strings.Builder
	b.WriteString("export")
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		b.WriteString(" " + name + "='" + strings.ReplaceAll(value, "'", `'\''`) + "'")
	}
	return b.String()
}
