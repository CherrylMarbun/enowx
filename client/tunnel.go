package client

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Tunnel is a connection to a port a session's processes listen on, on
// the daemon's host, through the daemon (tunnel.open). It is a net.Conn,
// so a local forward takes it as any other; deadlines aren't kept.
type Tunnel struct {
	c  *Client
	id uint32

	mu       sync.Mutex
	cond     *sync.Cond
	buf      []byte // received, not read yet
	credit   int    // bytes it may still send
	remote   bool   // the daemon closed it
	local    bool   // Close was called
	closeOne sync.Once
}

// OpenTunnel connects to port of session's processes through the daemon.
func (c *Client) OpenTunnel(session string, port int) (*Tunnel, error) {
	var info proto.TunnelInfo
	if err := c.Call(proto.MethodTunnelOpen, proto.TunnelOpenParams{Session: session, Port: port}, &info); err != nil {
		return nil, err
	}
	t := &Tunnel{c: c, id: info.Stream, credit: proto.TunnelWindow}
	t.cond = sync.NewCond(&t.mu)
	c.mu.Lock()
	early := c.early[info.Stream]
	delete(c.early, info.Stream)
	if c.tunnels == nil {
		c.tunnels = map[uint32]*Tunnel{}
	}
	c.tunnels[info.Stream] = t
	c.mu.Unlock()
	for _, f := range early {
		t.deliver(f)
	}
	return t, nil
}

// deliver takes a frame the daemon sent on the tunnel.
func (t *Tunnel) deliver(f proto.Frame) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch f.Type {
	case proto.TypeTunnelData:
		t.buf = append(t.buf, f.Payload...)
	case proto.TypeTunnelAck:
		t.credit += proto.DecodeAck(f.Payload)
	case proto.TypeTunnelClose:
		t.remote = true
	}
	t.cond.Broadcast()
}

// hangup ends the tunnel with the connection.
func (t *Tunnel) hangup() {
	t.mu.Lock()
	t.remote = true
	t.cond.Broadcast()
	t.mu.Unlock()
}

// Read reads what the port sent, giving the daemon back the credit.
func (t *Tunnel) Read(p []byte) (int, error) {
	t.mu.Lock()
	for len(t.buf) == 0 && !t.remote && !t.local {
		t.cond.Wait()
	}
	if len(t.buf) == 0 {
		t.mu.Unlock()
		return 0, io.EOF
	}
	n := copy(p, t.buf)
	t.buf = t.buf[n:]
	t.mu.Unlock()
	_ = t.c.write(proto.Frame{Type: proto.TypeTunnelAck, Channel: t.id, Payload: proto.EncodeAck(n)})
	return n, nil
}

// Write sends p to the port, within the credit the daemon gives back.
func (t *Tunnel) Write(p []byte) (int, error) {
	sent := 0
	for sent < len(p) {
		t.mu.Lock()
		for t.credit == 0 && !t.remote && !t.local {
			t.cond.Wait()
		}
		if t.remote || t.local {
			t.mu.Unlock()
			return sent, net.ErrClosed
		}
		part := min(len(p)-sent, t.credit, proto.TunnelChunk)
		t.credit -= part
		t.mu.Unlock()
		if err := t.c.write(proto.Frame{Type: proto.TypeTunnelData, Channel: t.id, Payload: p[sent : sent+part]}); err != nil {
			return sent, err
		}
		sent += part
	}
	return sent, nil
}

// Close ends the tunnel.
func (t *Tunnel) Close() error {
	t.closeOne.Do(func() {
		t.mu.Lock()
		t.local = true
		remote := t.remote
		t.cond.Broadcast()
		t.mu.Unlock()
		t.c.mu.Lock()
		delete(t.c.tunnels, t.id)
		if t.c.gone == nil {
			t.c.gone = map[uint32]bool{}
		}
		t.c.gone[t.id] = true
		t.c.mu.Unlock()
		if !remote {
			_ = t.c.write(proto.Frame{Type: proto.TypeTunnelClose, Channel: t.id})
		}
	})
	return nil
}

type tunnelAddr struct{}

func (tunnelAddr) Network() string { return "enowx-tunnel" }
func (tunnelAddr) String() string  { return "daemon" }

func (t *Tunnel) LocalAddr() net.Addr              { return tunnelAddr{} }
func (t *Tunnel) RemoteAddr() net.Addr             { return tunnelAddr{} }
func (t *Tunnel) SetDeadline(time.Time) error      { return nil }
func (t *Tunnel) SetReadDeadline(time.Time) error  { return nil }
func (t *Tunnel) SetWriteDeadline(time.Time) error { return nil }
