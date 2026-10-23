//go:build tsnet

package hostconn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"net/http/httptest"
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

	"github.com/enowx/enowx/core/hosts"
)

// A host with Tailnet set is signed in to over enowX's own tailnet
// device: the server, on the tailnet only, sees enowX's SSH sign-in and
// the commands that set it up.
func TestConnectThroughOwnTailnet(t *testing.T) {
	netns.SetEnabled(false)
	defer netns.SetEnabled(true)
	derpMap := integration.RunDERPAndSTUN(t, logger.Discard, "127.0.0.1")
	control := &testcontrol.Server{DERPMap: derpMap, DNSConfig: &tailcfg.DNSConfig{Proxied: true},
		MagicDNSDomain: "tail-scale.ts.net", Logf: logger.Discard}
	control.HTTPTestServer = httptest.NewUnstartedServer(control)
	control.HTTPTestServer.Start()
	defer control.HTTPTestServer.Close()
	tailnetControl = control.HTTPTestServer.URL
	defer func() { tailnetControl = "" }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	ran := make(chan string, 16)
	go serveSSH(t, ln, ran)

	t.Setenv("HOME", t.TempDir())
	h := hosts.Host{Name: "devbox", Address: "root@" + status.TailscaleIPs[0].String(), Tailnet: true}
	opts := Options{TrustHost: func(string, string) bool { return true },
		TailnetLogin: func(url string) { t.Logf("sign in at %s", url) }}
	defer func() {
		if own.tn != nil {
			own.tn.Close()
			own.tn = nil
		}
	}()
	// The server here runs no shell, so setting up fails after signing in;
	// what matters is that it was reached, and over the tailnet only.
	for range 20 {
		if _, err = connectRemote(h, opts); len(ran) > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	select {
	case cmd := <-ran:
		t.Logf("server ran %q (connect: %v)", cmd, err)
	default:
		t.Fatalf("server never reached: %v", err)
	}
}

// serveSSH lets root in with any key or none and tells ran what each exec
// asked for, answering it with a failure.
func serveSSH(t *testing.T, ln net.Listener, ran chan<- string) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Error(err)
		return
	}
	cfg := &ssh.ServerConfig{NoClientAuth: true}
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
						select {
						case ran <- string(req.Payload[4:]):
						default:
						}
						_ = req.Reply(true, nil)
						fmt.Fprintln(ch.Stderr(), "no shell here")
						_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{1}))
						ch.Close()
					}
				}()
			}
		}()
	}
}
