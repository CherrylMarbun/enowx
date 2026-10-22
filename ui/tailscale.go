package ui

import (
	"context"
	"errors"
	"os/user"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/tailnet"
)

// loadPeers lists the tailnet's devices; it runs off the main thread.
func (a *App) loadPeers() {
	peers, err := tailnet.Peers(context.Background())
	a.win.Update(func() {
		a.peers, a.peersErr, a.peersLoaded = peers, "", true
		switch {
		case errors.Is(err, tailnet.ErrNotInstalled):
			a.peersErr = "Tailscale isn't installed on this device. Install it to pick your servers from your tailnet here."
		case err != nil:
			a.peersErr = "Couldn't read Tailscale: " + strings.TrimPrefix(err.Error(), "tailnet: ")
		}
	})
}

// tailnetNote says why the switcher lists no tailnet device, if it doesn't.
func (a *App) tailnetNote() string {
	switch {
	case !a.peersLoaded:
		return "Looking for Tailscale…"
	case a.peersErr != "":
		return a.peersErr
	case len(a.peers) == 0:
		return "No other devices on your tailnet."
	case len(a.newPeers()) == 0:
		return "Every device on your tailnet is saved above."
	}
	return ""
}

func (a *App) openAddHost() {
	a.addHost = addHostForm{open: true}
	go a.loadPeers()
}

// savedPeer returns the saved host that points at a tailnet device.
func (a *App) savedPeer(p tailnet.Peer) (hosts.Host, bool) {
	for _, h := range a.hostList {
		if strings.Contains(h.Address, p.Address()) || (p.IP != "" && strings.Contains(h.Address, p.IP)) {
			return h, true
		}
	}
	return hosts.Host{}, false
}

// newPeers are the tailnet's devices not saved as hosts yet.
func (a *App) newPeers() []tailnet.Peer {
	var out []tailnet.Peer
	for _, p := range a.peers {
		if _, saved := a.savedPeer(p); !saved {
			out = append(out, p)
		}
	}
	return out
}

// connectPeer saves a tailnet device as a host, signing in as the tailnet
// user, and connects to it; a device saved already is just opened.
func (a *App) connectPeer(p tailnet.Peer) {
	if h, ok := a.savedPeer(p); ok {
		a.addHost.open = false
		a.connect(h)
		return
	}
	if a.opts.Hosts == nil {
		return
	}
	if strings.TrimSpace(a.tailUser) == "" {
		a.tailUser = localUser()
	}
	h, err := a.opts.Hosts.Add(hosts.Host{Name: p.HostName, Address: strings.TrimSpace(a.tailUser) + "@" + p.Address()})
	if err != nil {
		a.addHost.err = err.Error()
		return
	}
	a.addHost.open = false
	a.reloadHosts()
	a.connect(h)
}

// tailnetSection lists the tailnet's devices in the Add VPS dialog, each
// one click away from being connected.
func (a *App) tailnetSection(c *ui.Context) {
	if a.tailUser == "" {
		a.tailUser = localUser()
	}
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Icon(c, iconTailscale).FontSize(14).TextColor(colTextSoft)
			ui.Text(c, "On your tailnet").FontSize(12).Bold().TextColor(colTextSoft).Grow(1)
			ui.Text(c, "Sign in as").FontSize(12).TextColor(colMuted)
			ui.TextInput(c, &a.tailUser).Font(monoFont).Width(120).Label("SSH user for tailnet devices")
		})
		ui.Scroll(c).FillWidth().MaxHeight(260).Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, p := range a.peers {
					a.peerRow(c.Key(p.Address()), p)
				}
			})
		})
	})
}

func (a *App) peerRow(c *ui.Context, p tailnet.Peer) {
	saved, isSaved := a.savedPeer(p)
	ui.Row(c).FillWidth().Gap(10).Padding(8, 10).Radius(8).Background(colRaised).Children(func() {
		ui.Box(c).Size(28, 28).Radius(7).Background(colSelected).Center().TextColor(colTextSoft).Children(func() {
			ui.Icon(c, iconServer).FontSize(14)
		})
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				if p.Online {
					dot(c, colDone, false, 7)
				} else {
					dot(c, colIdle, true, 7)
				}
				ui.Text(c, p.HostName).Bold().SingleLine()
			})
			detail := p.OS + " · " + p.Address()
			if !p.Online {
				detail = "offline · " + detail
			}
			ui.Text(c, detail).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
		})
		label := "Connect"
		if isSaved {
			label = "Open"
		}
		var b ui.Element
		if p.Online {
			b = primaryButton(c, iconServer, label, "")
		} else {
			b = ghostButton(c, label)
		}
		if b.Clicked() {
			if isSaved {
				a.addHost.open = false
				a.connect(saved)
			} else {
				a.connectPeer(p)
			}
		}
	})
}

func localUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "root"
}
