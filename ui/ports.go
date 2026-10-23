package ui

import (
	"net"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
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
		Forwarded: a.forwardAddr,
		Forward:   func(p proto.Port) { a.forwardPort(s.ID, p) },
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

// forward is a port of the connected server forwarded here, for a
// session: it ends with the session.
type forward struct {
	l       net.Listener
	session string
}

// forwardAddr is where port of the connected server shows here, if it is
// forwarded.
func (a *App) forwardAddr(port int) (string, bool) {
	f, ok := a.forwards[port]
	if !ok {
		return "", false
	}
	return f.l.Addr().String(), true
}

// forwardPort forwards port p of the connected server to this machine, for
// session.
func (a *App) forwardPort(session string, p proto.Port) {
	cl := a.client
	if cl == nil || a.forwards[p.Port] != nil {
		return
	}
	host := p.Addr
	if host == "*" || host == "" {
		host = "127.0.0.1"
	}
	target := net.JoinHostPort(host, strconv.Itoa(p.Port))
	l, err := transport.Forward(p.Port, func() (net.Conn, error) { return a.dialPort(cl, session, p.Port, target) })
	if err != nil {
		a.status = "Couldn't forward port " + strconv.Itoa(p.Port) + ": " + err.Error()
		return
	}
	if a.forwards == nil {
		a.forwards = map[int]*forward{}
	}
	a.forwards[p.Port] = &forward{l: l, session: session}
	a.notify("Port " + strconv.Itoa(p.Port) + " of " + a.host.Name + " is at " + l.Addr().String())
}

func (a *App) unforwardPort(port int) {
	if f := a.forwards[port]; f != nil {
		f.l.Close()
		delete(a.forwards, port)
	}
}

// stopForwards ends the forwards of session, or every one for "".
func (a *App) stopForwards(session string) {
	for port, f := range a.forwards {
		if session == "" || f.session == session {
			a.unforwardPort(port)
		}
	}
}

// dialPort reaches a port of session on the connected server: through
// SSH, else through the daemon (tunnel.open), for SSH servers that don't
// forward ports; after one refusal, the connection's next ones go through
// the daemon at once. It runs off the main thread.
func (a *App) dialPort(cl *client.Client, session string, port int, target string) (net.Conn, error) {
	if !a.sshRefused.Load() {
		nc, err := cl.DialHost(target)
		if err == nil {
			return nc, nil
		}
		t, terr := cl.OpenTunnel(session, port)
		if terr != nil {
			return nil, err // a daemon without tunnels: SSH's reason
		}
		a.sshRefused.Store(true)
		return t, nil
	}
	return cl.OpenTunnel(session, port)
}
