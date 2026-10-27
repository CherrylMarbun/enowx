package transport

import (
	"errors"
	"os"
	"syscall"
)

// stopProcess ends the process; Windows has no polite signal for it.
func stopProcess(p *os.Process) error {
	if !running(p.Pid) {
		return nil
	}
	err := p.Kill()
	if err == nil || errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	// If the process has already exited by the time TerminateProcess is called,
	// Windows returns ERROR_ACCESS_DENIED (5).
	if !running(p.Pid) {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 5 && !running(p.Pid) {
		return nil
	}
	return err
}
