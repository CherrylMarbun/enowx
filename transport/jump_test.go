package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"

	"golang.org/x/crypto/ssh"
)

// testServer is an SSH server on loopback that takes any password: a
// jump host forwards connections (direct-tcpip); another runs commands,
// answering what it ran.
func testServer(t *testing.T, jump bool) string {
	t.Helper()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }}
	cfg.AddHostKey(signer)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			nc, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					go serveChannel(ch, jump)
				}
			}()
		}
	}()
	return l.Addr().String()
}

func serveChannel(nch ssh.NewChannel, jump bool) {
	switch {
	case jump && nch.ChannelType() == "direct-tcpip":
		var to struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if ssh.Unmarshal(nch.ExtraData(), &to) != nil {
			_ = nch.Reject(ssh.ConnectionFailed, "bad request")
			return
		}
		out, err := net.Dial("tcp", net.JoinHostPort(to.Host, strconv.Itoa(int(to.Port))))
		if err != nil {
			_ = nch.Reject(ssh.ConnectionFailed, err.Error())
			return
		}
		ch, reqs, err := nch.Accept()
		if err != nil {
			out.Close()
			return
		}
		go ssh.DiscardRequests(reqs)
		go func() { _, _ = io.Copy(out, ch); out.Close() }()
		_, _ = io.Copy(ch, out)
		ch.Close()
	case !jump && nch.ChannelType() == "session":
		ch, reqs, err := nch.Accept()
		if err != nil {
			return
		}
		for req := range reqs {
			if req.Type != "exec" {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			n := binary.BigEndian.Uint32(req.Payload)
			_, _ = ch.Write([]byte("ran: " + string(req.Payload[4:4+n])))
			_, _ = ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			ch.Close()
			return
		}
	default:
		_ = nch.Reject(ssh.UnknownChannelType, "not here")
	}
}

// A host behind a jump host is reached through it.
func TestDialThroughAJumpHost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")
	jump, target := testServer(t, true), testServer(t, false)
	host, port, _ := net.SplitHostPort(target)
	opts := SSHOptions{
		Password:  func() (string, error) { return "pw", nil },
		TrustHost: func(string, string) bool { return true },
	}
	c, err := DialSSH(SSHTarget{User: "u", Host: host, Port: port, Jump: "u@" + jump}, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out, err := c.Run("echo hi", nil)
	if err != nil || out != "ran: echo hi" {
		t.Fatalf("ran %q, %v", out, err)
	}
	if c.jump == nil {
		t.Fatal("not through the jump host")
	}
}
