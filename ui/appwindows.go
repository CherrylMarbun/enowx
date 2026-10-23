package ui

import (
	"net"
	"strconv"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/appwindow"
)

// Desktop apps a session runs on its display (enowx display, or Run with a
// display) show in windows of their own, as apps of this device (package
// appwindow): one opens by itself when the display does, and shows with
// the app's first window. They close with the session or the connection.

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
		Addr: addr,
		Name: a.sessionName(s) + " · " + a.host.Name,
		Show: show,
		OnClosed: func() {
			if a.appWindows[s.ID] == w {
				delete(a.appWindows, s.ID)
			}
		},
	})
	a.appWindows[s.ID] = w
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
