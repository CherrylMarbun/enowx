package transport

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

// A server can accept the connection and then say nothing, or stop halfway
// through signing in, and x/crypto's Timeout covers only the TCP connect:
// the handshake gets handshakeWait, and signInWait once the server says
// something, since Tailscale SSH's check waits for the user in a browser.
var (
	handshakeWait = 30 * time.Second
	signInWait    = 10 * time.Minute
)

// handshakeTimer closes a connection whose SSH handshake runs past its time.
type handshakeTimer struct {
	t       *time.Timer
	wait    time.Duration
	expired atomic.Bool
}

// extend gives the handshake signInWait from now, unless it already ran out.
func (h *handshakeTimer) extend() {
	if h != nil && h.t.Stop() {
		h.wait = signInWait
		h.t.Reset(signInWait)
	}
}

// clientOver signs in over nc within the handshake's time; *timer is the
// handshake's while it runs, for the banner callback to extend.
func clientOver(nc net.Conn, addr string, cfg *ssh.ClientConfig, timer **handshakeTimer) (*ssh.Client, error) {
	h := &handshakeTimer{wait: handshakeWait}
	h.t = time.AfterFunc(handshakeWait, func() {
		h.expired.Store(true)
		nc.Close()
	})
	*timer = h
	c, chans, reqs, err := ssh.NewClientConn(nc, addr, cfg)
	stopped := h.t.Stop()
	*timer = nil
	if err == nil && !stopped && h.expired.Load() {
		c.Close()
		err = net.ErrClosed
	}
	if err != nil {
		nc.Close()
		if h.expired.Load() {
			return nil, fmt.Errorf("no answer while signing in for %s", h.wait)
		}
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}
