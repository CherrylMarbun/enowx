package transport

import (
	"net"
	"strings"
	"testing"
	"time"
)

// A server that takes the connection and never answers gives up after
// the handshake's time, instead of leaving "Connecting…" forever.
func TestSilentServerGivesUp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	wait := handshakeWait
	handshakeWait = 300 * time.Millisecond
	defer func() { handshakeWait = wait }()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close() // held open, silent
		}
	}()
	host, port, _ := net.SplitHostPort(l.Addr().String())
	done := make(chan error, 1)
	go func() {
		_, err := DialSSH(SSHTarget{User: "u", Host: host, Port: port}, SSHOptions{TrustHost: func(string, string) bool { return true }})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "no answer") {
			t.Fatalf("err = %v, want no answer", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still signing in to a silent server")
	}
}
