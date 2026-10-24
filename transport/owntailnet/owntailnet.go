// Package owntailnet joins the user's tailnet as a device of enowX's own,
// without the Tailscale app, so the app reaches servers there. It is
// apart from transport so the server daemon doesn't carry Tailscale.
package owntailnet

import (
	"context"
	"fmt"
	"net"
	"time"

	"tailscale.com/tsnet"
)

// Tailnet is this app on the user's tailnet, as a device of its own,
// without the Tailscale app: servers on the tailnet are reached through
// it. Its state (the device's key) lives in Dir.
type Tailnet struct {
	srv *tsnet.Server
}

// Options say how to join.
type Options struct {
	Dir      string // where the device's state is kept
	Hostname string // its name on the tailnet
	// AuthKey joins without signing in; empty asks the user to, at the
	// URL Login gets.
	AuthKey string
	// ControlURL is the coordination server, "" for Tailscale's own.
	ControlURL string
	// Login is given the URL to sign the device in at, when it needs it.
	Login func(url string)
}

// Join joins the user's tailnet, waiting for them to sign the
// device in when it isn't yet, until ctx ends.
func Join(ctx context.Context, o Options) (*Tailnet, error) {
	srv := &tsnet.Server{Dir: o.Dir, Hostname: o.Hostname, AuthKey: o.AuthKey, ControlURL: o.ControlURL,
		Logf: func(string, ...any) {}}
	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("owntailnet: %w", err)
	}
	lc, err := srv.LocalClient()
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("owntailnet: %w", err)
	}
	told := ""
	for {
		st, err := lc.Status(ctx)
		if err == nil {
			if st.BackendState == "Running" {
				return &Tailnet{srv: srv}, nil
			}
			if st.AuthURL != "" && o.Login == nil {
				srv.Close()
				return nil, fmt.Errorf("owntailnet: not signed in")
			}
			if st.AuthURL != "" && st.AuthURL != told {
				told = st.AuthURL
				o.Login(st.AuthURL)
			}
		}
		select {
		case <-ctx.Done():
			srv.Close()
			return nil, fmt.Errorf("owntailnet: not signed in: %w", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Dial connects to addr on the tailnet.
func (t *Tailnet) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return t.srv.Dial(ctx, network, addr)
}

// Close leaves the tailnet.
func (t *Tailnet) Close() error { return t.srv.Close() }
