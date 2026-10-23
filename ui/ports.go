package ui

import (
	"net"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/ui/portview"
)

// The Ports tab of the side panel (see package portview). Forwards of a
// server's ports belong to the connection: they go on while the tab
// doesn't show, and stop with the connection.

// portsTab draws the Ports tab for session s.
func (a *App) portsTab(c *ui.Context, s proto.SessionInfo) {
	if a.portViews == nil {
		a.portViews = map[string]*portview.View{}
	}
	v := a.portViews[s.ID]
	if v == nil {
		v = portview.New()
		a.portViews[s.ID] = v
	}
	gen := a.gen
	v.Draw(c, portview.Options{
		Client:  a.client,
		Session: s,
		Host:    a.host.Name,
		Update:  a.onMain,
		Wanted: func() bool {
			return gen == a.gen && a.panelShows(panelPorts) && a.selected == s.ID && a.portViews[s.ID] == v
		},
		Forwarded: func(port int) (string, bool) {
			l, ok := a.forwards[port]
			if !ok {
				return "", false
			}
			return l.Addr().String(), true
		},
		Forward:   a.forwardPort,
		Unforward: a.unforwardPort,
		OpenURL:   func(url string) { go func() { _ = mygo.Shell.OpenExternal(url) }() },
		Preview:   a.previewFunc(s),
	})
}

// previewFunc previews a port of session s, nil without a web view.
func (a *App) previewFunc(s proto.SessionInfo) func(proto.Port) {
	if !webViewAvailable() {
		return nil
	}
	return func(p proto.Port) { a.previewPort(s, p) }
}

// forwardPort forwards port p of the connected server to this machine.
func (a *App) forwardPort(p proto.Port) {
	cl := a.client
	if cl == nil || a.forwards[p.Port] != nil {
		return
	}
	host := p.Addr
	if host == "*" || host == "" {
		host = "127.0.0.1"
	}
	target := net.JoinHostPort(host, strconv.Itoa(p.Port))
	l, err := transport.Forward(p.Port, func() (net.Conn, error) { return cl.DialHost(target) })
	if err != nil {
		a.status = "Couldn't forward port " + strconv.Itoa(p.Port) + ": " + err.Error()
		return
	}
	if a.forwards == nil {
		a.forwards = map[int]net.Listener{}
	}
	a.forwards[p.Port] = l
	a.notify("Port " + strconv.Itoa(p.Port) + " of " + a.host.Name + " is at " + l.Addr().String())
}

func (a *App) unforwardPort(port int) {
	if l := a.forwards[port]; l != nil {
		l.Close()
		delete(a.forwards, port)
	}
}

// stopForwards ends the connection's forwards.
func (a *App) stopForwards() {
	for port := range a.forwards {
		a.unforwardPort(port)
	}
}
