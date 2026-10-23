package daemon

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"

	"github.com/enowx/enowx/proto"
)

// A tunnel connects a client to a port a session's processes listen on
// (tunnel.open), through the connection itself: for hosts whose SSH server
// doesn't forward ports, and so the daemon reaches only sessions' ports.
// Its pump goroutine reads the port and sends, within the credit the
// client gives back; its writer goroutine writes what the client sent,
// then acknowledges it, so the connection's read loop never waits on the
// port.
type tunnel struct {
	id   uint32
	conn *conn
	nc   net.Conn

	mu      sync.Mutex
	cond    *sync.Cond
	credit  int      // bytes it may still send the client
	writes  [][]byte // from the client, for the writer
	closing bool
	once    sync.Once
}

// openTunnel dials port p of session id's processes for the connection.
func (c *conn) openTunnel(p proto.TunnelOpenParams) (uint32, error) {
	all, err := c.srv.allPorts()
	if err != nil {
		return 0, err
	}
	i := slices.IndexFunc(all[p.Session], func(port proto.Port) bool { return port.Port == p.Port })
	if i < 0 {
		return 0, fmt.Errorf("tunnel: session %s doesn't listen on %d", p.Session, p.Port)
	}
	nc, err := dialPort(all[p.Session][i])
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		nc.Close()
		return 0, errors.New("connection closed")
	}
	c.nextStream++
	t := &tunnel{id: c.nextStream, conn: c, nc: nc, credit: proto.TunnelWindow}
	t.cond = sync.NewCond(&t.mu)
	if c.tunnels == nil {
		c.tunnels = map[uint32]*tunnel{}
	}
	c.tunnels[t.id] = t
	c.mu.Unlock()
	go t.pump()
	go t.writer()
	return t.id, nil
}

// dialPort connects to a listener on this host, at the loopback address
// it takes.
func dialPort(p proto.Port) (net.Conn, error) {
	hosts := []string{"127.0.0.1", "::1"}
	if p.Addr == "::1" {
		hosts = []string{"::1", "127.0.0.1"}
	}
	var err error
	for _, h := range hosts {
		var nc net.Conn
		if nc, err = net.Dial("tcp", net.JoinHostPort(h, strconv.Itoa(p.Port))); err == nil {
			return nc, nil
		}
	}
	return nil, fmt.Errorf("tunnel: %w", err)
}

func (c *conn) tunnel(id uint32) *tunnel {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tunnels[id]
}

// pump sends what the port says to the client, within the credit.
func (t *tunnel) pump() {
	buf := make([]byte, proto.TunnelChunk)
	for {
		n, err := t.nc.Read(buf)
		for sent := 0; sent < n; {
			t.mu.Lock()
			for t.credit == 0 && !t.closing {
				t.cond.Wait()
			}
			if t.closing {
				t.mu.Unlock()
				return
			}
			part := min(n-sent, t.credit)
			t.credit -= part
			t.mu.Unlock()
			t.conn.send(proto.Frame{Type: proto.TypeTunnelData, Channel: t.id, Payload: append([]byte(nil), buf[sent:sent+part]...)})
			sent += part
		}
		if err != nil {
			t.close(true)
			return
		}
	}
}

// writer writes what the client sent to the port, and acknowledges it.
func (t *tunnel) writer() {
	for {
		t.mu.Lock()
		for len(t.writes) == 0 && !t.closing {
			t.cond.Wait()
		}
		if t.closing {
			t.mu.Unlock()
			return
		}
		p := t.writes[0]
		t.writes = t.writes[1:]
		t.mu.Unlock()
		if _, err := t.nc.Write(p); err != nil {
			t.close(true)
			return
		}
		t.conn.send(proto.Frame{Type: proto.TypeTunnelAck, Channel: t.id, Payload: proto.EncodeAck(len(p))})
	}
}

// take handles a tunnel frame from the client.
func (t *tunnel) take(f proto.Frame) {
	switch f.Type {
	case proto.TypeTunnelData:
		t.mu.Lock()
		t.writes = append(t.writes, f.Payload)
		t.cond.Broadcast()
		t.mu.Unlock()
	case proto.TypeTunnelAck:
		t.mu.Lock()
		t.credit += proto.DecodeAck(f.Payload)
		t.cond.Broadcast()
		t.mu.Unlock()
	case proto.TypeTunnelClose:
		t.close(false)
	}
}

// close ends the tunnel, telling the client unless it asked.
func (t *tunnel) close(tell bool) {
	t.once.Do(func() {
		t.mu.Lock()
		t.closing = true
		t.cond.Broadcast()
		t.mu.Unlock()
		t.nc.Close()
		t.conn.mu.Lock()
		delete(t.conn.tunnels, t.id)
		t.conn.mu.Unlock()
		if tell {
			t.conn.send(proto.Frame{Type: proto.TypeTunnelClose, Channel: t.id})
		}
	})
}
