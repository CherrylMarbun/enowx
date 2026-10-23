package procstat

import (
	"encoding/hex"
	"net"
	"slices"
	"strconv"
	"strings"
)

// Listener is a TCP port a process listens on.
type Listener struct {
	PID  int
	Port int
	Addr string // the address it listens on: "127.0.0.1", "::", "*" (all)
}

// Loopback reports whether only this machine reaches the listener.
func (l Listener) Loopback() bool {
	return strings.HasPrefix(l.Addr, "127.") || l.Addr == "::1" || l.Addr == "localhost"
}

// sortListeners orders listeners by port, then address, dropping repeats
// (a process listening on IPv4 and IPv6 alike keeps its first).
func sortListeners(list []Listener) []Listener {
	slices.SortFunc(list, func(a, b Listener) int {
		if a.Port != b.Port {
			return a.Port - b.Port
		}
		return strings.Compare(a.Addr, b.Addr)
	})
	return slices.CompactFunc(list, func(a, b Listener) bool { return a.Port == b.Port && a.PID == b.PID })
}

// splitHostPort splits "127.0.0.1:3000", "[::1]:3000" or "*:3000".
func splitHostPort(s string) (string, int, bool) {
	i := strings.LastIndexByte(s, ':')
	if i < 0 {
		return "", 0, false
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return "", 0, false
	}
	return strings.Trim(s[:i], "[]"), port, true
}

// parseLsof reads lsof -Fpn: "p<pid>" lines, each followed by "n<addr>"
// lines.
func parseLsof(out string) []Listener {
	var listeners []Listener
	pid := 0
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			pid, _ = strconv.Atoi(line[1:])
		case strings.HasPrefix(line, "n") && pid > 0:
			if addr, port, ok := splitHostPort(line[1:]); ok {
				listeners = append(listeners, Listener{PID: pid, Port: port, Addr: addr})
			}
		}
	}
	return sortListeners(listeners)
}

// parseProcAddr reads "0100007F:1F90": the address in hex, in the
// machine's byte order by 32-bit words, and the port.
func parseProcAddr(s string) (string, int, bool) {
	a, p, ok := strings.Cut(s, ":")
	if !ok {
		return "", 0, false
	}
	port, err := strconv.ParseUint(p, 16, 16)
	if err != nil {
		return "", 0, false
	}
	raw, err := hex.DecodeString(a)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return "", 0, false
	}
	for i := 0; i < len(raw); i += 4 {
		raw[i], raw[i+1], raw[i+2], raw[i+3] = raw[i+3], raw[i+2], raw[i+1], raw[i]
	}
	ip := net.IP(raw)
	if ip.IsUnspecified() {
		return "*", int(port), true
	}
	return ip.String(), int(port), true
}
