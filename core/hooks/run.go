package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// DefaultTimeout bounds a hook's command that sets none.
const DefaultTimeout = 30 * time.Second

// outputMax is how much of a command's output is kept: the end of it.
const outputMax = 8 << 10

// ExitBlock is the exit code with which a hook's command refuses the
// action, what it printed to stderr being the reason, as agents' own hooks
// do.
const ExitBlock = 2

// Run runs hook h on event ev: a refusal or context at once, a command in
// the event's folder with ev as JSON on its stdin and env added to its
// environment. A command printing and exiting 0 gives context; exiting
// ExitBlock refuses.
func Run(h proto.Hook, ev proto.HookEvent, env []string) (proto.HookRun, proto.HookDecision) {
	run := proto.HookRun{Hook: h.ID, Name: h.Name, Event: ev.Event, Session: ev.Session, Agent: ev.Agent, Start: time.Now()}
	var d proto.HookDecision
	switch h.Action {
	case proto.HookActionBlock:
		d.Block, d.Reason = true, h.Reason
		if d.Reason == "" {
			d.Reason = "Refused by the hook “" + name(h) + "”."
		}
		run.Blocked, run.Output = true, d.Reason
	case proto.HookActionContext:
		d.Context, run.Output = h.Context, h.Context
	case proto.HookActionRun:
		stdout, stderr, code, err := command(h, ev, env)
		run.Exit, run.Output = code, tail(strings.TrimSpace(stdout+"\n"+stderr))
		switch {
		case err != nil:
			run.Error = err.Error()
		case code == ExitBlock:
			d.Block, d.Reason, run.Blocked = true, strings.TrimSpace(stderr), true
			if d.Reason == "" {
				d.Reason = strings.TrimSpace(stdout)
			}
			if d.Reason == "" {
				d.Reason = "Refused by the hook “" + name(h) + "”."
			}
		case code == 0:
			d.Context = strings.TrimSpace(stdout)
		default:
			run.Error = fmt.Sprintf("exited with %d", code)
		}
	}
	run.Ms = time.Since(run.Start).Milliseconds()
	return run, d
}

// command runs h's command on ev, within its timeout.
func command(h proto.Hook, ev proto.HookEvent, env []string) (stdout, stderr string, code int, err error) {
	var in bytes.Buffer
	enc := json.NewEncoder(&in)
	enc.SetEscapeHTML(false) // commands read as typed: > and & stay
	if err := enc.Encode(ev); err != nil {
		return "", "", -1, err
	}
	timeout := DefaultTimeout
	if h.Timeout > 0 {
		timeout = time.Duration(h.Timeout) * time.Second
	}
	cmd := shellCommand(h.Command)
	if fi, err := os.Stat(ev.Dir); err == nil && fi.IsDir() {
		cmd.Dir = ev.Dir
	}
	cmd.Env = append(append(os.Environ(), env...),
		"ENOWX_HOOK_EVENT="+ev.Event, "ENOWX_HOOK_NAME="+name(h), "ENOWX_HOOK_SESSION="+ev.Session, "ENOWX_HOOK_AGENT="+ev.Agent)
	cmd.Stdin = &in
	cmd.WaitDelay = time.Second // a child still holding its output
	var out, errOut capped
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Start(); err != nil {
		return "", "", -1, fmt.Errorf("start: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-time.After(timeout):
		killGroup(cmd)
		<-done
		return out.String(), errOut.String(), -1, fmt.Errorf("timed out after %s", timeout)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.String(), errOut.String(), exit.ExitCode(), nil
	}
	if err != nil {
		return out.String(), errOut.String(), -1, err
	}
	return out.String(), errOut.String(), 0, nil
}

func name(h proto.Hook) string {
	if h.Name != "" {
		return h.Name
	}
	return h.Event
}

// capped keeps what is written up to a bound, dropping the rest.
type capped struct{ b bytes.Buffer }

func (c *capped) Write(p []byte) (int, error) {
	if room := 64<<10 - c.b.Len(); room > 0 {
		c.b.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (c *capped) String() string { return c.b.String() }

// tail is the end of s, at most outputMax.
func tail(s string) string {
	if len(s) <= outputMax {
		return s
	}
	return "…" + s[len(s)-outputMax:]
}
