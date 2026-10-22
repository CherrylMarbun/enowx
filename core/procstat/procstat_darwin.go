package procstat

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func processes() ([]Proc, error) {
	out, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,rss=,time=").Output()
	if err != nil {
		return nil, err
	}
	var list []Proc
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			continue
		}
		pid, _ := strconv.Atoi(f[0])
		ppid, _ := strconv.Atoi(f[1])
		rss, _ := strconv.ParseUint(f[2], 10, 64)
		list = append(list, Proc{PID: pid, PPID: ppid, RSS: rss << 10, CPUTime: parseCPUTime(f[3])})
	}
	return list, nil
}

// parseCPUTime reads ps's time: [[dd-]hh:]mm:ss.cc.
func parseCPUTime(s string) time.Duration {
	var days int
	if d, rest, ok := strings.Cut(s, "-"); ok {
		days, _ = strconv.Atoi(d)
		s = rest
	}
	parts := strings.Split(s, ":")
	var total float64
	for _, p := range parts {
		v, _ := strconv.ParseFloat(p, 64)
		total = total*60 + v
	}
	return time.Duration((total + float64(days)*86400) * float64(time.Second))
}

func memory() (total, used uint64) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, 0
	}
	total, _ = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	vm, err := exec.Command("vm_stat").Output()
	if err != nil {
		return total, 0
	}
	page := uint64(4096)
	var free uint64
	for _, line := range strings.Split(string(vm), "\n") {
		if i := strings.Index(line, "page size of "); i >= 0 {
			if n, err := strconv.ParseUint(strings.Fields(line[i+13:])[0], 10, 64); err == nil {
				page = n
			}
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, _ := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(v), "."), 10, 64)
		switch k {
		case "Pages free", "Pages inactive", "Pages speculative", "Pages purgeable":
			free += n
		}
	}
	if free*page > total {
		return total, 0
	}
	return total, total - free*page
}

// Env returns the value of key in pid's environment, which ps shows to
// the process's own user.
func Env(pid int, key string) string {
	out, err := exec.Command("ps", "-E", "-ww", "-o", "command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	for _, word := range strings.Fields(string(out)) {
		if v, ok := strings.CutPrefix(word, key+"="); ok {
			return v
		}
	}
	return ""
}
