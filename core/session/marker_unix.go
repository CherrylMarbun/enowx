//go:build !windows

package session

import (
	"os"
	"runtime"
)

// marked runs argv, or the login shell when empty, under /bin/sh named
// after session id (see markerPrefix). The shell waits for it rather than
// exec it, so the name stays; its INT trap keeps it alive when Ctrl+C
// reaches it with the program, which gets INT back at its default.
func marked(id string, argv []string) []string {
	if len(argv) == 0 {
		return []string{"/bin/sh", "-c", `trap : INT; "$1" -l; exit $?`, marker(id), loginShell()}
	}
	return append([]string{"/bin/sh", "-c", `trap : INT; "$@"; exit $?`, marker(id)}, argv...)
}

// loginShell is the user's shell.
func loginShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/sh"
}
