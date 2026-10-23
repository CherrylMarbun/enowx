package ui

import (
	"net"
	"strconv"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/appwindow"
)

// Desktop apps a session runs on its display (enowx display, or Run with a
// display) show in windows of their own, as apps of this device (package
// appwindow): one opens by itself when the display does, and shows with
// the app's first window. Closing one quits the app: its display stops,
// with all that runs on it. Disconnecting or quitting enowX only closes
// the windows; the apps go on on their host.

// openAppWindow shows the app on display port p of session s in its
// window, opening one unless it has one; show shows it before the app has
// a window.
func (a *App) openAppWindow(s proto.SessionInfo, p proto.Port, show bool) {
	if w := a.appWindows[s.ID]; w != nil {
		if show {
			w.Focus()
		}
		return
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(p.Port))
	if a.client != nil && a.client.Remote() {
		a.forwardPort(s.ID, p)
		local, ok := a.forwardAddr(p.Port)
		if !ok {
			return
		}
		addr = local
	}
	if a.appWindows == nil {
		a.appWindows = map[string]*appwindow.Window{}
	}
	var w *appwindow.Window
	w = appwindow.Open(appwindow.Options{
		Addr:  addr,
		Name:  a.sessionName(s) + " · " + a.host.Name,
		Show:  show,
		Scale: s.DisplayScale,
		OnClosed: func(byUser bool) {
			if a.appWindows[s.ID] == w {
				delete(a.appWindows, s.ID)
			}
			if byUser {
				a.quitApp(s.ID, p.Port)
			}
		},
	})
	a.appWindows[s.ID] = w
}

// quitApp stops the app on display port of session id, as closing its
// window quits it: the display and everything on it (the app, its dev
// server) stop, the session's agent or shell goes on.
func (a *App) quitApp(id string, port int) {
	cl := a.client
	if cl == nil {
		return
	}
	a.unforwardPort(port)
	go func() {
		if err := cl.StopDisplay(id, port); err != nil {
			a.onMain(func() { a.status = client.Explain(err) })
		}
	}()
}

// openWindowFunc opens session s's app in its window, nil where app
// windows can't show.
func (a *App) openWindowFunc(s proto.SessionInfo) func(proto.Port) {
	if !appwindow.Available() {
		return nil
	}
	return func(p proto.Port) { a.openAppWindow(s, p, true) }
}

// closeAppWindows closes session's app window, or every one for "".
func (a *App) closeAppWindows(session string) {
	for id, w := range a.appWindows {
		if session == "" || id == session {
			delete(a.appWindows, id)
			w.Close()
		}
	}
}
