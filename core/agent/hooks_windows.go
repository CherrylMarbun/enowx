//go:build windows

package agent

// hooksSupported: the hook commands are POSIX shell, which agents on
// Windows may not run them in; the screen tells their state instead.
func hooksSupported() bool { return false }
