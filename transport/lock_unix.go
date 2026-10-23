//go:build !windows

package transport

import (
	"os"
	"syscall"
	"time"
)

// lockFile takes an exclusive lock on f, waiting up to wait for a holder
// to let go.
func lockFile(f *os.File, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for {
		if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// running reports whether process pid still exists.
func running(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
