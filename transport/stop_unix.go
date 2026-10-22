//go:build !windows

package transport

import (
	"os"
	"syscall"
)

// stopProcess asks the process to exit, so a daemon removes its socket.
func stopProcess(p *os.Process) error { return p.Signal(syscall.SIGTERM) }
