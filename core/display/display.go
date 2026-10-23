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
	// Scale is how many pixels apps draw per point there, as on the
	// screen of the device that shows them, so they show sharp: 2 for a
	// Retina screen.
	Scale int
	stop  func()
}

// Env is the environment that sends a program's windows there, drawn at
// the display's scale by GTK and Qt apps.
func (d *Display) Env() []string {
	env := []string{"DISPLAY=:" + strconv.Itoa(d.Number)}
	if d.Scale > 1 {
		s := strconv.Itoa(d.Scale)
		env = append(env, "GDK_SCALE="+s, "QT_SCALE_FACTOR="+s, "QT_AUTO_SCREEN_SCALE_FACTOR=0")
	}
	return env
}

// Stop ends the display, and the windows on it.
func (d *Display) Stop() {
	if d != nil && d.stop != nil {
		d.stop()
	}
}

// ErrUnsupported says this host can't run displays, and why.
var ErrUnsupported = errors.New("display: unsupported")
