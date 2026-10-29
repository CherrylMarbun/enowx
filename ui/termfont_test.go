package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// Settings' Terminal section lists this device's monospaced fonts, and
// picking one makes it the terminals' font.
func TestFontSectionPicksTheTerminalsFont(t *testing.T) {
	a := New(Options{})
	// The device's own fonts, rather than asking the system for them.
	a.fonts = []ui.FontFamily{{Name: "Fira Code", Monospaced: true}, {Name: "Test Mono", Monospaced: true}}
	a.fontsLoaded = true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(520).Height(420).Padding(20).Gap(16).Background(theme.Background).Children(func() {
			a.fontSection(c)
		})
	}, 520, 420)
	tt.Frame()
	if !tt.HasText("Test Mono") || !tt.HasText("System monospaced font") {
		t.Fatalf("the fonts are not listed: %q", tt.Texts())
	}
	if err := tt.Click("Test Mono"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.state.TermFont != "Test Mono" {
		t.Fatalf("picked %q, not Test Mono", a.state.TermFont)
	}
	if f := a.termFont(); f.Family != "Test Mono" {
		t.Fatalf("the terminals take %q", f.Family)
	}
	if err := tt.Click("System monospaced font"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.state.TermFont != "" {
		t.Fatalf("the system's font leaves %q", a.state.TermFont)
	}
}

// A device with no monospaced font of its own says so, and is not asked
// again: the section is drawn every frame.
func TestFontSectionWithNoMonospacedFont(t *testing.T) {
	a := New(Options{})
	a.fontsLoaded = true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(520).Height(420).Padding(20).Gap(16).Background(theme.Background).Children(func() {
			a.fontSection(c)
		})
	}, 520, 420)
	tt.Frame()
	if !tt.HasText("This device has no other monospaced font.") {
		t.Fatalf("the section does not say the device has none: %q", tt.Texts())
	}
	if a.fontsLoading {
		t.Fatal("the fonts are read again, once a frame")
	}
}
