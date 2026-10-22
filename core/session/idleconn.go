package session

import (
	"io"
	"sync"
)

// idleConn is the Conn of a terminal that runs nothing: reads wait until it
// closes, writes (typing) are dropped.
type idleConn struct {
	once sync.Once
	done chan struct{}
}

func newIdleConn() *idleConn { return &idleConn{done: make(chan struct{})} }

func (c *idleConn) Read([]byte) (int, error) {
	<-c.done
	return 0, io.EOF
}

func (c *idleConn) Write(p []byte) (int, error) { return len(p), nil }

func (c *idleConn) Close() error {
	c.once.Do(func() { close(c.done) })
	return nil
}
