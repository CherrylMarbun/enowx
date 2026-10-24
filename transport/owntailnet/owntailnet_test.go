package owntailnet

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"tailscale.com/net/netns"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
	"tailscale.com/tstest/integration"
	"tailscale.com/tstest/integration/testcontrol"
	"tailscale.com/types/logger"

	"github.com/enowx/enowx/transport"
)

// enowX joins a tailnet as a device of its own and signs in to a server
// there over SSH, the server reached by its tailnet address only: a
// coordination server and a relay of the tailnet's run here.
func TestSSHOverOwnTailnet(t *testing.T) {
	netns.SetEnabled(false)
	defer netns.SetEnabled(true)
	derpMap := integration.RunDERPAndSTUN(t, logger.Discard, "127.0.0.1")
	control := &testcontrol.Server{DERPMap: derpMap, DNSConfig: &tailcfg.DNSConfig{Proxied: true},
		MagicDNSDomain: "tail-scale.ts.net", Logf: logger.Discard}
	control.HTTPTestServer = httptest.NewUnstartedServer(control)
	control.HTTPTestServer.Start()
	defer control.HTTPTestServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The server: a tailnet device with an SSH server on port 22.
	server := &tsnet.Server{Dir: filepath.Join(t.TempDir(), "server"), Hostname: "devbox",
		ControlURL: control.HTTPTestServer.URL, Ephemeral: true, Logf: logger.Discard}
	defer server.Close()
	status, err := server.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := server.Listen("tcp", ":22")
	if err != nil {
		t.Fatal(err)
	}
	go serveSSH(t, ln)

	// enowX, joining as a device of its own.
	home := t.TempDir()
	t.Setenv("HOME", home)
	tn, err := Join(ctx, Options{Dir: filepath.Join(home, "tsnet"), Hostname: "enowx-test",
		ControlURL: control.HTTPTestServer.URL, Login: func(url string) { t.Logf("sign in at %s", url) }})
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()

	addr := status.TailscaleIPs[0].String()
	target, err := transport.ParseSSHTarget("ada@" + addr)
	if err != nil {
		t.Fatal(err)
	}
	target.Port = "22"
	var c *transport.SSHClient
	for range 20 { // the peers learn of each other in a moment
		c, err = transport.DialSSH(target, transport.SSHOptions{
			Dial:      tn.Dial,
			Password:  func() (string, error) { return "secret", nil },
			TrustHost: func(string, string) bool { return true },
		})
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	out, err := c.Run("hello", nil)
	if err != nil || out != "ran: hello\n" {
		t.Fatalf("ran %q, %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(home, "tsnet")); err != nil {
		t.Fatalf("no tailnet state kept: %v", err)
	}
}

// serveSSH answers SSH on ln: user ada, password secret, and each exec
// says what it was asked to run.
func serveSSH(t *testing.T, ln net.Listener) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Error(err)
		return
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(m ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
		if m.User() == "ada" && string(pw) == "secret" {
			return nil, nil
		}
		return nil, fmt.Errorf("no")
	}}
	cfg.AddHostKey(signer)
	for {
		nc, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
			if err != nil {
				return
			}
			go ssh.DiscardRequests(reqs)
			for nch := range chans {
				ch, requests, err := nch.Accept()
				if err != nil {
					continue
				}
				go func() {
					for req := range requests {
						if req.Type != "exec" {
							_ = req.Reply(false, nil)
							continue
						}
						cmd := string(req.Payload[4:])
						_ = req.Reply(true, nil)
						fmt.Fprintf(ch, "ran: %s\n", cmd)
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						ch.Close()
					}
				}()
			}
		}()
	}
}
