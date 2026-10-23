package procstat

import "golang.org/x/sys/windows"

// Disk returns the size and the space used of the volume holding path, in
// bytes; zeros when it can't tell.
func Disk(path string) (total, used uint64) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	var free, size, totalFree uint64
	if windows.GetDiskFreeSpaceEx(p, &free, &size, &totalFree) != nil {
		return 0, 0
	}
	return size, size - free
}
