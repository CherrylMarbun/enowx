//go:build !windows

package main

// daemonExe is the program this machine's daemon runs as: the app's own,
// which an update replaces while it runs, as Unix allows.
func daemonExe(exe, dir string) string { return exe }
