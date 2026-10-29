package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/enowx/enowx/proto"
)

// conn is one client connection. Its read loop runs on the goroutine that
// called serve; writes come from many goroutines and are serialized by wmu.
type conn struct {
	srv *Server
	nc  net.Conn

	wmu sync.Mutex

	mu         sync.Mutex
	streams    map[uint32]*stream
	tunnels    map[uint32]*tunnel
	adb        adbBridge
	nextStream uint32
	closed     bool
	// greeted is set once the handshake is done: events reach the
	// connection only after, as its client reads none before.
	greeted atomic.Bool
	// remote: it came through the team listener, a visitor's, served once
	// visitor is set (visitors.go).
	remote  bool
	visitor atomic.Pointer[proto.Visitor]
}

func newConn(s *Server, nc net.Conn) *conn {
	return &conn{srv: s, nc: nc, streams: map[uint32]*stream{}}
}

func (c *conn) serve() error {
	defer c.close()
	if err := c.handshake(); err != nil {
		return err
	}
	c.greeted.Store(true)
	for {
		f, err := proto.ReadFrame(c.nc)
		if err != nil {
			return err
		}
		switch f.Type {
		case proto.TypeRequest:
			var req proto.Request
			if err := json.Unmarshal(f.Payload, &req); err != nil {
				return fmt.Errorf("bad request: %w", err)
			}
			// Requests run in order on this goroutine, so a resize sent
			// after an attach always finds its stream; slow ones, which
			// no other request depends on, run beside it.
			if slowMethods[req.Method] {
				go func() {
					value, err := c.handle(req)
					c.reply(req.ID, value, err)
				}()
				continue
			}
			value, err := c.handle(req)
			c.reply(req.ID, value, err)
		case proto.TypeInput:
			if st := c.stream(f.Channel); st != nil {
				st.input(f.Payload)
			}
		case proto.TypeTunnelData, proto.TypeTunnelAck, proto.TypeTunnelClose:
			if t := c.tunnel(f.Channel); t != nil {
				t.take(f)
			}
		default:
			return fmt.Errorf("unexpected %s frame", f.Type)
		}
	}
}

func (c *conn) handshake() error {
	f, err := proto.ReadFrame(c.nc)
	if err != nil {
		return err
	}
	if f.Type != proto.TypeHello {
		return fmt.Errorf("want hello, got %s", f.Type)
	}
	var h proto.Hello
	if err := json.Unmarshal(f.Payload, &h); err != nil {
		return fmt.Errorf("bad hello: %w", err)
	}
	v, ok := proto.Negotiate(h.Protocol)
	if !ok {
		return fmt.Errorf("protocol %d not supported (daemon speaks %d to %d)", h.Protocol, proto.MinVersion, proto.Version)
	}
	payload, _ := json.Marshal(proto.Hello{Protocol: v, Agent: "enowxd/" + Version})
	c.send(proto.Frame{Type: proto.TypeHello, Payload: payload})
	return nil
}

func (c *conn) attach(id string) (uint32, error) {
	sess, err := c.srv.sessions.Get(id)
	if err != nil {
		return 0, err
	}
	m, err := sess.Terminal().NewMirror()
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		m.Close()
		return 0, errors.New("connection closed")
	}
	c.nextStream++
	st := newStream(c, c.nextStream, sess, m)
	c.streams[st.id] = st
	c.mu.Unlock()
	go st.pump()
	return st.id, nil
}

func (c *conn) detach(id uint32) {
	c.mu.Lock()
	st := c.streams[id]
	delete(c.streams, id)
	c.mu.Unlock()
	if st != nil {
		st.stop()
	}
}

func (c *conn) stream(id uint32) *stream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streams[id]
}

// send writes a frame; errors surface in the read loop as the socket closes.
func (c *conn) send(f proto.Frame) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := proto.WriteFrame(c.nc, f); err != nil {
		_ = c.nc.Close()
	}
}

func (c *conn) close() {
	c.mu.Lock()
	c.closed = true
	streams := c.streams
	c.streams = map[uint32]*stream{}
	tunnels := c.tunnels
	c.tunnels = nil
	c.mu.Unlock()
	for _, st := range streams {
		st.stop()
	}
	for _, t := range tunnels {
		t.close(false)
	}
	c.closeADB()
	_ = c.nc.Close()
}
