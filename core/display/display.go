// Package display runs virtual displays for sessions on Linux servers, so
// desktop apps (Tauri, Electron, GTK, Qt, Flutter) show their windows
// there, and the app shows them through Xpra's HTML5 client, served on a
// loopback port it forwards, in windows of their own (ui/appwindow) or the
// Browser tool. See docs/preview.md.
package display

import (
	"errors"
	"strconv"
)

// Display is a session's virtual display.
type Display struct {
	Number int // the X display, as DISPLAY=:Number
	Port   int // the loopback port serving its HTML5 client
	stop   func()
}

// Env is the environment variable that sends a program's windows there.
func (d *Display) Env() string { return "DISPLAY=:" + strconv.Itoa(d.Number) }

// Stop ends the display, and the windows on it.
func (d *Display) Stop() {
	if d != nil && d.stop != nil {
		d.stop()
	}
}

// ErrUnsupported says this host can't run displays, and why.
var ErrUnsupported = errors.New("display: unsupported")
