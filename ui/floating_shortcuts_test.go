package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// The shortcut and Settings turn the floating control on and off.
func TestToggleFloating(t *testing.T) {
	app := New(Options{})
	if app.state.FloatingControl {
		t.Fatal("on at start")
	}
	app.toggleFloating()
	if !app.state.FloatingControl {
		t.Fatal("toggleFloating didn't turn it on")
	}
	app.toggleFloating()
	if app.state.FloatingControl {
		t.Fatal("toggleFloating didn't turn it off")
	}
}

// The global shortcut summons and folds the panel; the feature stays on.
func TestFloatingShortcutSummons(t *testing.T) {
	ready, register, bound := floatingShortcutsReady, registerFloatingShortcut, floatingShortcutBound
	defer func() {
		floatingShortcutsReady, registerFloatingShortcut, floatingShortcutBound = ready, register, bound
	}()
	floatingShortcutsReady = func() bool { return true }
	floatingShortcutBound = func(string) bool { return false }
	var press func()
	registerFloatingShortcut = func(_ string, fn func()) error { press = fn; return nil }

	app := New(Options{})
	app.setFloatingControl(true)
	if press == nil {
		t.Fatal("the shortcut wasn't bound")
	}
	if !app.state.FloatingControl {
		t.Fatal("the feature isn't on")
	}
	if !app.floatingShown {
		t.Fatal("turning it on doesn't show the panel once")
	}
	press() // folds it away, as Escape would
	if app.floatingShown {
		t.Fatal("the shortcut doesn't fold the panel away")
	}
	if !app.state.FloatingControl {
		t.Fatal("the shortcut turned the feature off")
	}
	press() // summons again, the key still bound
	if !app.floatingShown {
		t.Fatal("the shortcut summons only once")
	}
	press() // and folds again
	if app.floatingShown {
		t.Fatal("the second press doesn't fold it")
	}
}

// A shortcut another app holds leaves the feature off, and says why.
func TestFloatingShortcutRefused(t *testing.T) {
	ready, register := floatingShortcutsReady, registerFloatingShortcut
	defer func() { floatingShortcutsReady, registerFloatingShortcut = ready, register }()
	floatingShortcutsReady = func() bool { return true }
	registerFloatingShortcut = func(string, func()) error { return errors.New("taken") }

	app := New(Options{})
	app.setFloatingControl(true)
	if app.state.FloatingControl {
		t.Fatal("on, though the key is another app's")
	}
	if app.status == "" {
		t.Fatal("the status bar doesn't say why")
	}
	if app.floatingShown {
		t.Fatal("the panel shows without its shortcut")
	}
}

// A blur in the moment the panel shows is the window still coming up, not the
// user going elsewhere: it must not fold the panel away before it was ever
// seen.
func TestFloatingBlurRightAfterShowIgnored(t *testing.T) {
	app := New(Options{})
	app.showFloating() // no window: the intent alone, as a test sees it
	if !app.floatingShown {
		t.Fatal("showFloating didn't raise the panel")
	}
	app.blurFloating()
	if !app.floatingShown {
		t.Fatal("the blur of the showing folded the panel away")
	}
}

// A blur once the panel has been up long enough to have been seen folds it:
// the user went elsewhere.
func TestFloatingBlurAfterShowFolds(t *testing.T) {
	app := New(Options{})
	app.floatingShown = true
	app.floatingShownAt = time.Now().Add(-time.Second)
	app.blurFloating()
	if app.floatingShown {
		t.Fatal("a blur a second after the summon didn't fold the panel")
	}
}

// Folding on blur is a setting: turned off, the user going elsewhere leaves
// the panel up, and the centre, Escape or the shortcut dismisses it.
func TestFloatingBlurKeptWhenFoldOff(t *testing.T) {
	app := New(Options{})
	off := false
	app.state.FloatingFoldOnBlur = &off
	app.floatingShown = true
	app.floatingShownAt = time.Now().Add(-time.Second)
	app.blurFloating()
	if !app.floatingShown {
		t.Fatal("the panel folded though folding is off")
	}
	app.hideFloating() // and the centre's way still works
	if app.floatingShown {
		t.Fatal("hideFloating left the panel up")
	}
}

// hideFloating folds the panel away, leaving floatingShown false, and does
// nothing to a panel that is already folded — neither needing a window.
func TestHideFloatingFolds(t *testing.T) {
	app := New(Options{})
	app.showFloating() // no window: the intent alone
	if !app.floatingShown {
		t.Fatal("showFloating didn't raise the panel")
	}
	app.hideFloating() // no window: it must not panic
	if app.floatingShown {
		t.Fatal("hideFloating left the panel up")
	}
	app.hideFloating() // already folded: still nothing, still no panic
	if app.floatingShown {
		t.Fatal("hideFloating raised a hidden panel")
	}
}

// A close the app asked for must not be held back: quitting closes the panel
// for good. One the user asked for folds it away instead.
func TestFloatingCloseWhileQuitting(t *testing.T) {
	app := New(Options{})
	if app.quitting {
		t.Fatal("quitting at start")
	}
	app.floatingShown = true

	quit := &mygo.CloseEvent{}
	app.quitting = true
	app.closeFloating(quit)
	if quit.DefaultPrevented() {
		t.Fatal("quitting is held back by the panel")
	}

	app.quitting = false
	app.floatingShown = true
	user := &mygo.CloseEvent{}
	app.closeFloating(user)
	if !user.DefaultPrevented() {
		t.Fatal("the panel closes instead of folding away")
	}
	if app.floatingShown {
		t.Fatal("the panel stays up after it folds")
	}
}

// Hiding the Sessions panel leaves the room to the session, and comes back.
func TestSessionsPanelHides(t *testing.T) {
	app := New(Options{})
	if app.sessionsHidden() {
		t.Fatal("hidden at start")
	}
	app.state.HideSessions = true
	if !app.sessionsHidden() {
		t.Fatal("state.HideSessions doesn't hide the panel")
	}
	app.state.HideSessions = false
	if app.sessionsHidden() {
		t.Fatal("the panel stays hidden")
	}
	app.toggleSessionsPanel()
	if !app.sessionsHidden() {
		t.Fatal("toggleSessionsPanel didn't hide it")
	}
	app.toggleSessionsPanel()
	if app.sessionsHidden() {
		t.Fatal("toggleSessionsPanel didn't bring it back")
	}
	// A maximized session still folds it away, whatever the setting says.
	app.maximized = "s1"
	if !app.sessionsHidden() {
		t.Fatal("a maximized session doesn't hide the panel")
	}
}
