//go:build !windows

package daemon

import (
	"syscall"
	"time"

	"github.com/enowx/enowx/core/procstat"
)

// reapGrace is how long processes get to exit after SIGTERM.
const reapGrace = 3 * time.Second

// tag is an environment entry that marks the processes something started
// (procstat.Tagged): a session's ENOWX_SESSION, a display's DISPLAY.
type tag struct{ key, value string }

// reap stops the processes tagged with t that outlive what started them: a
// dev server an agent left in tmux after its session is deleted, a
// display's D-Bus after the display stops. They get SIGTERM, then SIGKILL
// after reapGrace, waited for on a goroutine of its own.
func reap(t tag) {
	if pids := terminate(t); len(pids) > 0 {
		go func() {
			time.Sleep(reapGrace)
			kill(t, pids)
		}()
	}
}

// reapAll is reap for every tag, returning once they are gone: the daemon
// is about to exit.
func reapAll(tags []tag) {
	left := map[tag][]int{}
	for _, t := range tags {
		if pids := terminate(t); len(pids) > 0 {
			left[t] = pids
		}
	}
	if len(left) == 0 {
		return
	}
	time.Sleep(reapGrace)
	for t, pids := range left {
		kill(t, pids)
	}
}

func terminate(t tag) []int {
	pids := procstat.Tagged(t.key, t.value)
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	return pids
}

// kill ends those of pids that still run and are still tagged, not a new
// process given a pid freed meanwhile.
func kill(t tag, pids []int) {
	for _, pid := range pids {
		if procstat.Env(pid, t.key) == t.value {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}
