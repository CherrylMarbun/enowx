package ui

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// floatingWindowOptions are the window the control lives in: a MyGo Panel,
// frameless and transparent, sized to the pie's canvas.
//
// Panel can't be traded away on Windows: it alone keeps the window out of
// Alt-Tab (WS_EX_TOOLWINDOW), on top from its creation (WS_EX_TOPMOST), and
// off every taskbar (an owner window); SkipTaskbar and AlwaysOnTop don't.
// Its cost is the hairline Windows 11 draws round a Panel, whose corners
// MyGo rounds: setUpFloating takes the rounding back (dropFloatingFrame).
func floatingWindowOptions(a *App) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title:       "Floating control",
		Panel:       true, // over other apps' windows, out of the taskbar and Alt-Tab
		Frameless:   true,
		Transparent: true,
		// A material is the only thing that gives the window per-pixel
		// alpha on Windows: noRedirect is fixed at creation, and without
		// it the cleared frame paints opaque black. It is dropped at once
		// by setUpFloating, so nothing blurs behind the pie.
		Vibrancy: mygo.VibrancyMica,
		Hidden:   true,
		// The pie draws its own shadow: the system's hugs the square the
		// window still is, not the slices.
		DisableShadow:     true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		SkipTaskbar:       true,
		Width:             a.floatingWindowSize(),
		Height:            a.floatingWindowSize(),
		Content:           ui.View(a.floatingView),
	}
}

// setUpFloating creates the panel, hidden, once the main window is open.
func (a *App) setUpFloating() {
	a.floating = mygo.NewWindow(floatingWindowOptions(a))
	// No material, only the alpha it unlocked: a panel that blurred what
	// is behind it was what the pie replaced.
	a.floating.SetVibrancy(mygo.VibrancyNone)
	// And no frame: a Panel's corners are rounded at creation, and DWM
	// outlines the window it rounds (floatingWindowOptions).
	dropFloatingFrame(a.floating)
	// Hidden rather than closed, so the window is reused: Escape, Cmd-W and
	// going elsewhere all fold the panel away, and quitting destroys it.
	a.floating.OnClose(a.closeFloating)
	// A panel can lose the foreground as it shows, before it ever held it:
	// fold only once it has been up long enough to have been seen.
	a.floating.OnBlur(a.blurFloating)
	// setUpTray registers this too, but it gives up before that when there
	// is no icon: the panel must know a quit is under way either way, or
	// its OnClose holds the quit back.
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.floating.Destroy() })
	a.applyFloatingShortcut()
}

// toggleFloating turns the floating control on or off: on, its panel shows
// once at the pointer; off, the panel folds away and the shortcut is freed
// for other apps. It is Settings' and the palette's switch, not the
// shortcut's (summonFloating), so pressing the key never takes the feature
// away.
func (a *App) toggleFloating() { a.setFloatingControl(!a.state.FloatingControl) }

// summonFloating shows the panel at the pointer, or folds it away when it
// is up: what the global shortcut does, the feature staying on.
func (a *App) summonFloating() {
	if a.floatingShown {
		a.hideFloating()
		return
	}
	a.showFloating()
}

// setFloatingControl turns the floating control on or off, from Settings or
// the palette. It stays off when the shortcut can't be taken, and the
// status bar says why.
func (a *App) setFloatingControl(on bool) {
	a.state.FloatingControl = on
	if on {
		if a.applyFloatingShortcut() != nil {
			return // the key is another app's: the switch stays off
		}
		a.showFloating()
		return
	}
	a.hideFloating()
	a.applyFloatingShortcut()
}

// applyFloatingShortcut binds or frees the global shortcut, as the setting
// says, and reports why it couldn't take it.
func (a *App) applyFloatingShortcut() error {
	if !floatingShortcutsReady() {
		return nil // before the app runs, and in tests
	}
	if !a.state.FloatingControl {
		unregisterFloatingShortcut(floatingShortcut)
		return nil
	}
	if floatingShortcutBound(floatingShortcut) {
		return nil
	}
	if err := registerFloatingShortcut(floatingShortcut, a.summonFloating); err != nil {
		a.state.FloatingControl = false
		a.status = "Couldn't take " + floatingKey + " for the floating control: " + err.Error()
		return err
	}
	return nil
}

// showFloating puts the panel at the pointer, or in the middle of the
// display when the setting says so, kept inside the work area of that
// display so it never hangs off the screen.
func (a *App) showFloating() {
	a.floatingShown, a.floatingFresh = true, true // the intent, which a test sees without a window
	a.floatingShownAt = time.Now()
	if a.floating == nil {
		return
	}
	p := mygo.Screen.CursorScreenPoint()
	d := mygo.Screen.DisplayNearestPoint(p)
	if !a.floatingAtPointer() {
		// The middle of the display, which the one clamp below places as
		// the pointer would be.
		p = mygo.Point{X: d.WorkArea.X + d.WorkArea.Width/2, Y: d.WorkArea.Y + d.WorkArea.Height/2}
	}
	size := a.floatingWindowSize()
	a.floating.SetSize(size, size)
	x := max(d.WorkArea.X, min(p.X-size/2, d.WorkArea.X+d.WorkArea.Width-size))
	y := max(d.WorkArea.Y, min(p.Y-size/2, d.WorkArea.Y+d.WorkArea.Height-size))
	a.floating.SetPosition(x, y)
	a.floating.Show()
	a.floating.Focus()
	a.floating.Invalidate() // the actions depend on the host and the project
}

// blurFloating folds the panel away when the user goes elsewhere. A panel
// can lose the foreground as it shows, before it ever held it: a blur
// inside the first moments is the window still coming up, and folds
// nothing, so a summon is not undone in the frame it was made. A test
// calls it without a window.
func (a *App) blurFloating() {
	if time.Since(a.floatingShownAt) < 250*time.Millisecond {
		return
	}
	if !a.floatingFoldOnBlur() {
		return // folding is off: the hub, Escape or the shortcut dismisses it
	}
	a.hideFloating()
}

// closeFloating folds the panel away rather than closing it, unless the app
// is quitting: a close the app asked for must not be held back, or the quit
// never finishes. A test calls it without a window.
func (a *App) closeFloating(e *mygo.CloseEvent) {
	if a.quitting {
		return // quitting closes it for good
	}
	e.PreventDefault()
	a.hideFloating()
}

// hideFloating folds the panel away. The view hides the window once it has
// folded, and the feature stays on, its shortcut bound.
func (a *App) hideFloating() {
	if !a.floatingShown {
		return
	}
	a.floatingShown = false
	if a.floating != nil {
		a.floating.Invalidate() // the view animates the fold, then hides the window
	}
}

// runFloating runs an action in the window, which the panel may have
// covered: the window comes forward and draws what the action changed.
func (a *App) runFloating(act floatingAction) {
	a.showWindow()
	act.run()
	if a.win != nil {
		a.win.Update(func() {}) // a state change here redraws there
	}
}
