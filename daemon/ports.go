package daemon

import (
	"path/filepath"
	"runtime"

	"github.com/enowx/enowx/core/procstat"
	"github.com/enowx/enowx/proto"
)

// ports lists the TCP ports session id's processes listen on.
func (s *Server) ports(id string) (proto.SessionPorts, error) {
	if _, err := s.sessions.Get(id); err != nil {
		return proto.SessionPorts{}, err
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return proto.SessionPorts{}, nil
	}
	s.stats() // samples the processes and finds each session's
	st := &s.statsState
	st.mu.Lock()
	var pids []int
	for pid, owner := range st.owners {
		if owner == id {
			pids = append(pids, st.sampler.Descendants(pid)...)
		}
	}
	st.mu.Unlock()
	listeners, err := procstat.Listening(pids)
	if err != nil {
		return proto.SessionPorts{}, err
	}
	out := proto.SessionPorts{Supported: true}
	for _, l := range listeners {
		name := ""
		if args := procstat.Args(l.PID); len(args) > 0 {
			name = filepath.Base(args[0])
		}
		out.Ports = append(out.Ports, proto.Port{Port: l.Port, Addr: l.Addr, PID: l.PID, Process: name})
	}
	return out, nil
}
