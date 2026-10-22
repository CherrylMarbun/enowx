package client

import (
	"io"
	"sync"

	"github.com/enowx/enowx/proto"
)

// Stream is one attachment to a session. It implements the Conn a terminal
// with ResizeByConn expects: Read returns the daemon's frames, Write sends
// typed input, and ResizePixels asks the daemon for a new size.
//
// Size changes from the daemon arrive in order with the frames. Read calls
// OnResize before returning the bytes drawn at the new size, so the
// terminal resizes exactly between the frames of the two sizes.
type Stream struct {
	c         *Client
	id        uint32
	SessionID string
	// OnResize is called on the reading goroutine with the daemon's new
	// size. Set it before the terminal starts reading.
	OnResize func(cols, rows int)

	mu     sync.Mutex
	cond   *sync.Cond
	queue  []proto.Frame
	closed bool
}

func newStream(c *Client, id uint32, sessionID string) *Stream {
	st := &Stream{c: c, id: id, SessionID: sessionID}
	st.cond = sync.NewCond(&st.mu)
	return st
}

func (st *Stream) deliver(f proto.Frame) {
	st.mu.Lock()
	if !st.closed {
		st.queue = append(st.queue, f)
		st.cond.Signal()
	}
	st.mu.Unlock()
}

func (st *Stream) hangup() {
	st.mu.Lock()
	st.closed = true
	st.cond.Broadcast()
	st.mu.Unlock()
}

// Read returns the next frame's bytes, applying size changes on the way.
func (st *Stream) Read(p []byte) (int, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for {
		for len(st.queue) == 0 && !st.closed {
			st.cond.Wait()
		}
		if len(st.queue) == 0 {
			return 0, io.EOF
		}
		f := st.queue[0]
		if f.Type == proto.TypeResize {
			st.queue = st.queue[1:]
			if cols, rows, err := proto.DecodeSize(f.Payload); err == nil && st.OnResize != nil {
				st.mu.Unlock()
				st.OnResize(cols, rows)
				st.mu.Lock()
			}
			continue
		}
		n := copy(p, f.Payload)
		if n < len(f.Payload) {
			st.queue[0].Payload = f.Payload[n:]
		} else {
			st.queue = st.queue[1:]
		}
		return n, nil
	}
}

// Write sends typed bytes to the session.
func (st *Stream) Write(p []byte) (int, error) {
	if err := st.c.write(proto.Frame{Type: proto.TypeInput, Channel: st.id, Payload: p}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Resize asks the daemon to resize the session.
func (st *Stream) Resize(cols, rows int) error {
	return st.ResizePixels(cols, rows, 0, 0)
}

// ResizePixels asks the daemon to resize the session, with the size of a
// cell in pixels for programs that ask.
func (st *Stream) ResizePixels(cols, rows, cellWidth, cellHeight int) error {
	return st.c.Call(proto.MethodSessionResize, proto.ResizeParams{
		Stream: st.id, Cols: cols, Rows: rows, CellWidth: cellWidth, CellHeight: cellHeight,
	}, nil)
}

// Clear asks the daemon to clear the session's screen.
func (st *Stream) Clear() error {
	return st.c.Call(proto.MethodSessionClear, proto.SessionRef{ID: st.SessionID}, nil)
}

// Close detaches from the session, which keeps running.
func (st *Stream) Close() error {
	st.hangup()
	st.c.forget(st.id)
	err := st.c.Call(proto.MethodSessionDetach, proto.StreamRef{Stream: st.id}, nil)
	if err == ErrClosed {
		return nil
	}
	return err
}
