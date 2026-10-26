package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Settings' sections show as tabs where they fit and as a menu where they
// don't, and a window that keeps its width keeps one of them: no flipping
// from frame to frame at any width.
func TestSettingsSectionsDontFlip(t *testing.T) {
	app := New(Options{})
	app.page = pageSettings
	for w := 600; w <= 1600; w += 10 {
		width := float32(w)
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Width(width).Height(500).Background(theme.Background).Children(func() { app.settingsPage(c) })
		}, w, 500)
		for range 4 {
			tt.Frame()
		}
		menu := tt.HasText("Settings section")
		for i := range 8 {
			tt.Frame()
			if tt.HasText("Settings section") != menu {
				t.Fatalf("at %d wide the sections flipped between tabs and a menu (frame %d)", w, i)
			}
		}
		if w <= 700 && !menu {
			t.Errorf("at %d wide the tabs show, squeezed", w)
		}
		if w >= 1600 && menu {
			t.Errorf("at %d wide the sections are a menu", w)
		}
	}
}
