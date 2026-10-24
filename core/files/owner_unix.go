//go:build !windows

package files

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// owner is "user:group" of a file, by name where the system knows them.
func owner(info os.FileInfo) string {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	u, g := strconv.Itoa(int(st.Uid)), strconv.Itoa(int(st.Gid))
	if found, err := user.LookupId(u); err == nil {
		u = found.Username
	}
	if found, err := user.LookupGroupId(g); err == nil {
		g = found.Name
	}
	return u + ":" + g
}
