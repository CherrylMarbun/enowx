// Package appwindow shows a desktop app that runs on a session's virtual
// display (core/display) in a window of its own on this device, as if the
// app ran here: the window holds nothing but the app, follows its title,
// and resizing it resizes the app. It loads the display's HTML5 client
// (Xpra's) in a native web view, so it works wherever enowX does.
package appwindow

import (
	_ "embed"
	"net/url"
	"strings"

	"github.com/egoist/mygo"
)

//go:embed shell.js
var shell string

// Titles the shell script gives the page (shell.js).
const (
	titleApp  = "enowx-app:"
	titleWait = "enowx-wait"
)

// Window is an app's window. It belongs to the main thread.
type Window struct {
	w      *mygo.Window
	closed bool
}

// Options open a Window.
type Options struct {
	// Addr is where the display's client is served here (host:port): its
	// port, forwarded for a server.
	Addr string
	// Name titles the window until the app gives its own.
	Name string
	// Show shows the window at once, before the app opens one: when the
	// user asked for it. Otherwise it shows with the app's first window.
	Show bool
	// OnClosed runs on the main thread once the user closed the window.
	OnClosed func()
}

// Available reports whether app windows can show here: they need a web
// view.
func Available() bool { return mygo.WebViewAvailable() }

// Open opens a window for the display at o.Addr.
func Open(o Options) *Window {
	q := url.Values{}
	for k, v := range map[string]string{
		"floating_menu": "false", "clock": "false", "sound": "false", "printing": "false",
		"file_transfer": "false", "remote_logging": "false", "reconnect": "true", "steal": "true",
	} {
		q.Set(k, v)
	}
	a := &Window{}
	a.w = mygo.NewWindow(mygo.WindowOptions{
		Title:           o.Name,
		URL:             "http://" + o.Addr + "/index.html?" + q.Encode(),
		Width:           1100,
		Height:          760,
		Hidden:          !o.Show,
		BackgroundColor: "#000000",
		Page:            mygo.PageOptions{PreloadScript: shell},
	})
	a.w.Page().OnPageTitleUpdated(func(e *mygo.TitleEvent) {
		e.PreventDefault()
		switch {
		case strings.HasPrefix(e.Title, titleApp):
			if t := strings.TrimPrefix(e.Title, titleApp); t != "" {
				a.w.SetTitle(t)
			}
			if !a.w.IsVisible() {
				a.w.Show()
			}
		case e.Title == titleWait && !o.Show:
			// No window: the app quit or restarts, as on a desktop.
			a.w.Hide()
		}
	})
	a.w.OnClosed(func() {
		a.closed = true
		if o.OnClosed != nil {
			o.OnClosed()
		}
	})
	return a
}

// Focus shows the window in front.
func (a *Window) Focus() {
	if a.closed {
		return
	}
	a.w.Show()
	a.w.Focus()
}

// Close closes the window; the app goes on running on its display.
func (a *Window) Close() {
	if !a.closed {
		a.w.Close()
	}
}
