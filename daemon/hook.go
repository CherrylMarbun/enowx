package daemon

import (
	"os"
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
		path, err := transport.SocketPath()
		if err != nil {
			return
		}
		nc, err := transport.DialLocal(path, nil)
		if err != nil {
			return
		}
		c, err := client.New(nc, nil)
		if err != nil {
			nc.Close()
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
