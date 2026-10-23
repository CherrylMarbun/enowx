//go:build !windows

package procstat

import "golang.org/x/sys/unix"

// Disk returns the size and the space used of the filesystem holding path,
// in bytes; zeros when it can't tell.
func Disk(path string) (total, used uint64) {
	var st unix.Statfs_t
	if unix.Statfs(path, &st) != nil {
		return 0, 0
	}
	bs := uint64(st.Bsize)
	total = st.Blocks * bs
	return total, total - st.Bavail*bs
}
