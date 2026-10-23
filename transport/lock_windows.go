package transport

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// lockFile takes an exclusive lock on f, waiting up to wait for a holder
// to let go.
func lockFile(f *os.File, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for {
		ol := new(windows.Overlapped)
		err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
		if err == nil {
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
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	ev, err := windows.WaitForSingleObject(h, 0)
	return err == nil && ev == uint32(windows.WAIT_TIMEOUT)
}
