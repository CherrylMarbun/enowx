package android

import (
	"bufio"
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// A device's log is read by `adb logcat` while someone looks at it, into
// a ring of its last lines; nobody asking for two minutes stops it.
const (
	logKeep = 5000
	logIdle = 2 * time.Minute
)

// Logs reads devices' logs. It is safe for use by several goroutines.
type Logs struct {
	mu      sync.Mutex
	readers map[string]*logReader
}

type logReader struct {
	mu     sync.Mutex
	lines  []proto.AndroidLogLine
	next   int64
	asked  time.Time
	cancel context.CancelFunc
	pids   map[string][]int // package → its processes, looked up now and then
	pidsAt map[string]time.Time
}

// threadtime is logcat's -v threadtime line: date, time, pid, tid, level,
// tag, message.
var threadtime = regexp.MustCompile(`^\d\d-\d\d\s+(\d\d:\d\d:\d\d\.\d+)\s+(\d+)\s+\d+\s+([VDIWEFA])\s+(.*?)\s*:\s(.*)$`)

func parseLogLine(line string) (proto.AndroidLogLine, bool) {
	m := threadtime.FindStringSubmatch(line)
	if m == nil {
		return proto.AndroidLogLine{}, false
	}
	pid, _ := strconv.Atoi(m[2])
	return proto.AndroidLogLine{Time: m[1], PID: pid, Level: m[3], Tag: m[4], Message: m[5]}, true
}

// Read returns device serial's lines after after, only pkg's when set,
// starting its reader the first time.
func (l *Logs) Read(s SDK, serial, pkg string, after int64) proto.AndroidLog {
	l.mu.Lock()
	if l.readers == nil {
		l.readers = map[string]*logReader{}
	}
	r := l.readers[serial]
	if r == nil {
		r = &logReader{pids: map[string][]int{}, pidsAt: map[string]time.Time{}}
		l.readers[serial] = r
		l.start(s, serial, r)
	}
	l.mu.Unlock()
	r.mu.Lock()
	r.asked = time.Now()
	var pids []int
	if pkg != "" {
		if time.Since(r.pidsAt[pkg]) > 3*time.Second {
			r.mu.Unlock()
			out, _ := s.shell(serial, "pidof "+pkg)
			var found []int
			for _, f := range strings.Fields(out) {
				if n, err := strconv.Atoi(f); err == nil {
					found = append(found, n)
				}
			}
			r.mu.Lock()
			r.pids[pkg], r.pidsAt[pkg] = found, time.Now()
		}
		pids = r.pids[pkg]
	}
	out := proto.AndroidLog{Lines: []proto.AndroidLogLine{}, Last: r.next}
	for _, line := range r.lines {
		if line.N <= after {
			continue
		}
		if pkg != "" && !slices.Contains(pids, line.PID) {
			continue
		}
		out.Lines = append(out.Lines, line)
	}
	r.mu.Unlock()
	return out
}

func (l *Logs) start(s SDK, serial string, r *logReader) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() {
		defer l.forget(serial, r)
		cmd := s.command(ctx, s.ADB(), "-s", serial, "logcat", "-v", "threadtime", "-T", "500")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return
		}
		if err := cmd.Start(); err != nil {
			return
		}
		go func() {
			for ctx.Err() == nil {
				time.Sleep(10 * time.Second)
				r.mu.Lock()
				idle := time.Since(r.asked) > logIdle
				r.mu.Unlock()
				if idle {
					cancel()
				}
			}
		}()
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			line, ok := parseLogLine(sc.Text())
			if !ok {
				continue
			}
			r.mu.Lock()
			r.next++
			line.N = r.next
			r.lines = append(r.lines, line)
			if len(r.lines) > logKeep {
				r.lines = slices.Delete(r.lines, 0, len(r.lines)-logKeep)
			}
			r.mu.Unlock()
		}
		cmd.Wait()
	}()
}

func (l *Logs) forget(serial string, r *logReader) {
	l.mu.Lock()
	if l.readers[serial] == r {
		delete(l.readers, serial)
	}
	l.mu.Unlock()
}

// Close stops every reader.
func (l *Logs) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, r := range l.readers {
		r.cancel()
	}
	l.readers = nil
}
