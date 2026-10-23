//go:build !windows

package transport

import (
	"os"
	"syscall"
)

// private reports whether dir is this user's alone, so nobody else can
// put a socket of theirs in it.
func private(dir string) bool {
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0o077 != 0 {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == os.Getuid()
}
