package procstat

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Listening returns the TCP ports the processes pids listen on: the
// listening sockets of /proc/net/tcp and tcp6 that those processes hold.
func Listening(pids []int) ([]Listener, error) {
	sockets := map[string]Listener{} // inode → listener
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n")[1:] {
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != "0A" { // 0A: LISTEN
				continue
			}
			addr, port, ok := parseProcAddr(f[1])
			if ok {
				sockets[f[9]] = Listener{Port: port, Addr: addr}
			}
		}
	}
	var out []Listener
	for _, pid := range pids {
		fds, _ := filepath.Glob("/proc/" + strconv.Itoa(pid) + "/fd/*")
		for _, fd := range fds {
			target, err := os.Readlink(fd)
			if err != nil || !strings.HasPrefix(target, "socket:[") {
				continue
			}
			if l, ok := sockets[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")]; ok {
				l.PID = pid
				out = append(out, l)
			}
		}
	}
	return sortListeners(out), nil
}
