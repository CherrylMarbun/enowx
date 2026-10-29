package daemon

import (
	"net"
	"sync"
	"time"
)

// What the team listener lets in before anyone is known: anyone who
// reaches the host's Tailscale address can try, so a connection gets
// little room, little time and few tries before its ticket.

var (
	// visitorSignInWait is how long a visitor's connection has to say
	// hello and hand its ticket.
	visitorSignInWait = 10 * time.Second
	// visitorPreAuthMax is the largest frame read before the ticket.
	visitorPreAuthMax = 64 << 10
)

const (
	teamConnsMax   = 128 // connections on the team listener at once
	teamConnsPerIP = 16  // of those, from one address
	authsPerIP     = 10  // tickets one address hands in authWindow
	authWindow     = time.Minute
)

// visitorGate counts the team listener's connections and tickets by
// remote address; its mutex guards all of it.
type visitorGate struct {
	mu    sync.Mutex
	total int
	conns map[string]int
	auths map[string][]time.Time
}

// admit takes a connection from ip in, unless too many are open.
func (g *visitorGate) admit(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.total >= teamConnsMax || g.conns[ip] >= teamConnsPerIP {
		return false
	}
	if g.conns == nil {
		g.conns = map[string]int{}
	}
	g.total++
	g.conns[ip]++
	return true
}

// release counts a connection from ip gone.
func (g *visitorGate) release(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.total--
	if g.conns[ip]--; g.conns[ip] <= 0 {
		delete(g.conns, ip)
	}
}

// allowAuth counts a ticket from ip, unless it handed too many lately.
func (g *visitorGate) allowAuth(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if g.auths == nil {
		g.auths = map[string][]time.Time{}
	}
	// Forget what is past the window, everyone's, so the map stays small.
	for k, list := range g.auths {
		i := 0
		for i < len(list) && now.Sub(list[i]) >= authWindow {
			i++
		}
		if i == len(list) {
			delete(g.auths, k)
		} else {
			g.auths[k] = list[i:]
		}
	}
	if len(g.auths[ip]) >= authsPerIP {
		return false
	}
	g.auths[ip] = append(g.auths[ip], now)
	return true
}

// remoteIP is the address a connection comes from, without its port.
func remoteIP(nc net.Conn) string {
	addr := nc.RemoteAddr()
	if addr == nil {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr.String()); err == nil {
		return host
	}
	return addr.String()
}
