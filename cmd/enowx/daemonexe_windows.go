package main

import "log"

// daemonExe is the program this machine's daemon runs as: on Windows a copy
// of the app's own (daemonCopy). The daemon outlives the app's window, and
// Windows locks a running program's files, so the installer and updates
// couldn't replace the app's while it ran ("Error opening file for
// writing").
func daemonExe(exe, dir string) string {
	p, err := daemonCopy(exe, dir)
	if err != nil {
		log.Printf("daemon: %v; it runs as the app", err)
		return exe
	}
	return p
}
