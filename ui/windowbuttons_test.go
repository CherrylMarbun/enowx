package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

type fakeWindow struct{ did []string }

func (w *fakeWindow) Close()          { w.did = append(w.did, "close") }
func (w *fakeWindow) Minimize()       { w.did = append(w.did, "minimize") }
func (w *fakeWindow) ToggleMaximize() { w.did = append(w.did, "maximize") }

// The title bar's own buttons close, minimize and maximize the window,
// the same on every system.
func TestWindowButtons(t *testing.T) {
	a := New(Options{})
	w := &fakeWindow{}
	a.winCtl = w
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(900).Height(60).Background(theme.Ground).Children(func() { a.titleBar(c) })
	}, 900, 60)
	tt.Frame()
	for _, b := range []string{"Zoom", "Minimize", "Close"} {
		if err := tt.Click(b); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	shot(t, tt, "title-bar.png")
	if got := w.did; len(got) != 3 || got[0] != "maximize" || got[1] != "minimize" || got[2] != "close" {
		t.Fatalf("did %q", got)
	}
}
