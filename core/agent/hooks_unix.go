//go:build !windows

package agent

// hooksSupported: hooks run a POSIX shell command.
func hooksSupported() bool { return true }
