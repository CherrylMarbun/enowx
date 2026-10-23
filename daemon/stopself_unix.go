//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// stopSelf stops the daemon as SIGTERM does: its sessions saved first.
func stopSelf() { _ = syscall.Kill(os.Getpid(), syscall.SIGTERM) }
