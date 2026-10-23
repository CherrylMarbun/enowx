package procstat

import (
	"os/exec"
	"strconv"
	"strings"
)

// Listening returns the TCP ports the processes pids listen on, as lsof
// shows them.
func Listening(pids []int) ([]Listener, error) {
	if len(pids) == 0 {
		return nil, nil
	}
	list := make([]string, len(pids))
	for i, p := range pids {
		list[i] = strconv.Itoa(p)
	}
	// -a: only those processes' TCP listeners. lsof fails when none is
	// found, which is no error here.
	out, _ := exec.Command("lsof", "-nP", "-a", "-p", strings.Join(list, ","), "-iTCP", "-sTCP:LISTEN", "-Fpn").Output()
	return parseLsof(string(out)), nil
}
