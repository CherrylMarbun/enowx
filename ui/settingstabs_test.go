package ui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Settings lists its sections on its left, grouped, every one of them
// there; picking one shows it. Narrow, the list keeps its icons and the
// sections stay reachable by their names.
func TestSettingsNavigation(t *testing.T) {
	app := New(Options{})
	app.page = pageSettings
	for _, w := range []int{1200, 640} {
		width := float32(w)
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Width(width).Height(700).Background(theme.Background).Children(func() { app.settingsPage(c) })
		}, w, 700)
		tt.Frame()
		tt.Frame()
		shot(t, tt, fmt.Sprintf("settings-nav-%d.png", w))
		list := settingsSections()
		for _, name := range list {
			if _, ok := tt.Find(name); !ok {
				t.Errorf("at %d wide %q isn't in the navigation", w, name)
			}
		}
		if len(list) != len(sectionIcons) {
			t.Fatalf("%d sections listed, %d with icons", len(list), len(sectionIcons))
		}
		if wide := tt.HasText("AGENTS"); wide != (w >= settingsNarrow) {
			t.Errorf("at %d wide the groups' labels show: %v", w, wide)
		}
		if err := tt.Click(sectionCloud); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if app.settingsSection != sectionCloud {
			t.Fatalf("at %d wide picking Cloud shows %q", w, app.settingsSection)
		}
		app.settingsSection = sectionLibrary
	}
}
