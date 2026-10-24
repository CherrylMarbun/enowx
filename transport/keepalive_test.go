package transport

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// A connection whose network went silent is closed after a few
// keepalives go unanswered; a live one stays.
func TestKeepalivesCloseADeadConnection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	every, missed := keepAliveEvery, keepAliveMissed
	keepAliveEvery, keepAliveMissed = 100*time.Millisecond, 3
	defer func() { keepAliveEvery, keepAliveMissed = every, missed }()

	server := testServer(t, false)
	// A proxy that can go silent, as a network does when the laptop
	// sleeps or changes Wi-Fi.
	var frozen atomic.Bool
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			in, err := l.Accept()
			if err != nil {
				return
			}
			out, err := net.Dial("tcp", server)
			if err != nil {
				in.Close()
				continue
			}
			pipe := func(dst, src net.Conn) {
				buf := make([]byte, 32<<10)
				for {
					n, err := src.Read(buf)
					if err != nil {
						dst.Close()
						return
					}
					if frozen.Load() {
						continue // dropped
					}
					if _, err := dst.Write(buf[:n]); err != nil {
						return
					}
				}
			}
			go pipe(out, in)
			go pipe(in, out)
		}
	}()
	host, port, _ := net.SplitHostPort(l.Addr().String())
	c, err := DialSSH(SSHTarget{User: "u", Host: host, Port: port}, SSHOptions{
		Password:  func() (string, error) { return "pw", nil },
		TrustHost: func(string, string) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	closed := make(chan struct{})
	go func() { _ = c.client.Wait(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("a live connection was closed")
	case <-time.After(500 * time.Millisecond):
	}
	frozen.Store(true)
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("a dead connection stayed open")
	}
}
