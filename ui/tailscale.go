package ui

import (
	"context"
	"os/user"
	"strings"

	"github.com/enowx/enowx/core/tailnet"
)

// loadPeers lists the tailnet's devices; it runs off the main thread.
// Without Tailscale the list stays empty.
func (a *App) loadPeers() {
	peers, err := tailnet.Peers(context.Background())
	a.win.Update(func() {
		if err == nil {
			a.peers = peers
		}
	})
}

// newPeers are the tailnet's devices not saved as hosts yet.
func (a *App) newPeers() []tailnet.Peer {
	var out []tailnet.Peer
	for _, p := range a.peers {
		saved := false
		for _, h := range a.hostList {
			if strings.Contains(h.Address, p.Address()) || (p.IP != "" && strings.Contains(h.Address, p.IP)) {
				saved = true
			}
		}
		if !saved {
			out = append(out, p)
		}
	}
	return out
}

func localUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "root"
}
