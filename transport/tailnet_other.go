//go:build !tsnet

package transport

import (
	"context"
	"errors"
	"net"
)

// TailnetBuilt says this build carries Tailscale (build tag tsnet); this
// one doesn't, and reaches tailnets through the device's own Tailscale.
const TailnetBuilt = false

// Tailnet is this app on the user's tailnet; builds without Tailscale have
// none.
type Tailnet struct{}

// TailnetOptions say how to join.
type TailnetOptions struct {
	Dir, Hostname, AuthKey, ControlURL string
	Login                              func(url string)
}

var errNoTailnet = errors.New("transport: this build of enowX doesn't carry Tailscale: use the device's Tailscale")

// JoinTailnet fails: this build doesn't carry Tailscale.
func JoinTailnet(context.Context, TailnetOptions) (*Tailnet, error) { return nil, errNoTailnet }

// Dial fails: this build doesn't carry Tailscale.
func (*Tailnet) Dial(context.Context, string, string) (net.Conn, error) { return nil, errNoTailnet }

// Close does nothing.
func (*Tailnet) Close() error { return nil }
