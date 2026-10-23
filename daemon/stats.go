package daemon

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/enowx/enowx/core/procstat"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// stats measures the daemon's host. Measurements closer together than
// minSample reuse the last one, so several windows asking cost one.
type stats struct {
	sampler procstat.Sampler

	mu     sync.Mutex
	last   time.Time
	owners map[int]string // the daemon's children → their session
	cached proto.DaemonStats
}

const minSample = time.Second

func (s *Server) stats() proto.DaemonStats {
	st := &s.statsState
	st.mu.Lock()
	defer st.mu.Unlock()
	if time.Since(st.last) < minSample {
		return st.cached
	}
	st.last = time.Now()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		st.cached = proto.DaemonStats{}
		return st.cached
	}
	if err := st.sampler.Sample(); err != nil {
		s.logger().Warn("could not measure processes", "err", err)
		return st.cached
	}
	if st.owners == nil {
		st.owners = map[int]string{}
	}
	// Each session runs as a child of the daemon, named after it.
	self := os.Getpid()
	children := st.sampler.Children(self)
	alive := map[int]bool{}
	for _, pid := range children {
		alive[pid] = true
		if _, known := st.owners[pid]; !known {
			st.owners[pid] = sessionOf(pid)
		}
	}
	for pid := range st.owners {
		if !alive[pid] {
			delete(st.owners, pid)
		}
	}
	h := st.sampler.Host()
	out := proto.DaemonStats{
		Supported: true,
		Host:      proto.HostUsage{CPUs: h.CPUs, MemTotal: h.MemTotal, MemUsed: h.MemUsed, CPU: h.CPU},
		Daemon:    usage(st.sampler.One(self)),
	}
	bySession := map[string]*proto.SessionUsage{}
	for _, pid := range children {
		id := st.owners[pid]
		if id == "" {
			continue
		}
		u := bySession[id]
		if u == nil {
			u = &proto.SessionUsage{ID: id}
			bySession[id] = u
		}
		t := st.sampler.Tree(pid)
		u.RSS, u.CPU, u.Procs = u.RSS+t.RSS, u.CPU+t.CPU, u.Procs+t.Procs
	}
	for _, sess := range s.sessions.List() {
		if u := bySession[sess.ID()]; u != nil {
			out.Sessions = append(out.Sessions, *u)
		}
	}
	st.cached = out
	return out
}

// sessionOf is the session a child of the daemon runs: as its arguments
// name it, else its environment (sessions of daemons before the name).
func sessionOf(pid int) string {
	if id := session.IDFromArgs(procstat.Args(pid)); id != "" {
		return id
	}
	return procstat.Env(pid, session.EnvSession)
}

func usage(u procstat.Usage) proto.Usage {
	return proto.Usage{RSS: u.RSS, CPU: u.CPU, Procs: u.Procs}
}
