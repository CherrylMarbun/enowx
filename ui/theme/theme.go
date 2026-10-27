// Package theme is enowX's look: the color tokens every view reads, the
// palettes that set them, fonts, and the themes of windows and terminals.
//
// The tokens are package variables read while drawing, and the UI is drawn
// every frame, so Apply changes the whole app at once. They belong to the
// main thread.
package theme

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// The color tokens. Status colors differ in lightness as well as hue.
var (
	Background   ui.Color // window, terminals
	Ground       ui.Color // what the bento panels sit on: darker than the window
	Panel        ui.Color // rail, session list, title and status bars
	Raised       ui.Color // cards
	Sunken       ui.Color // inputs, tracks: below the surface
	Hover        ui.Color
	Selected     ui.Color
	Border       ui.Color // hairlines between areas
	BorderStrong ui.Color // outlines of controls
	Dialog       ui.Color // dialogs, menus, pop-overs
	Text         ui.Color
	TextStrong   ui.Color
	TextSoft     ui.Color
	Muted        ui.Color
	Accent       ui.Color
	AccentHigh   ui.Color // the light end of the accent's gradient
	AccentSoft   ui.Color // accent tint behind selected options
	AccentLine   ui.Color // accent outline of the selected item
	OnAccent     ui.Color
	Highlight    ui.Color // the faint light along a raised surface's top

	Working ui.Color
	Done    ui.Color
	Warning ui.Color
	Error   ui.Color
	Idle    ui.Color
)

// MonoFont is the font of paths, code and command output.
const MonoFont = "monospace"

// Shape: the same in every palette.
const (
	RadiusControl = 9  // buttons, inputs, rows
	RadiusCard    = 12 // cards, menus
	RadiusPanel   = 16 // dialogs, sections
	RadiusBento   = 14 // the window's panels
	BentoGap      = 8  // between the window's panels, and around them
	ControlHeight = 32
)

// current is the palette applied last.
var current Palette

// Current is the palette in use.
func Current() Palette { return current }

func init() { Apply(Champagne) }

// Apply sets every token from p.
func Apply(p Palette) {
	current = p
	Background, Panel, Raised, Sunken = p.Background, p.Panel, p.Raised, p.Sunken
	Ground = Mix(p.Background, ui.Hex("#000000"), 0.5)
	if p.Light {
		Ground = Mix(p.Background, ui.Hex("#000000"), 0.035)
	}
	Hover, Selected, Dialog = p.Hover, p.Selected, p.Dialog
	Border, BorderStrong = p.Border, p.BorderStrong
	Text, TextStrong, TextSoft, Muted = p.Text, p.TextStrong, p.TextSoft, p.Muted
	Accent, OnAccent = p.Accent, p.OnAccent
	AccentHigh = Mix(p.Accent, ui.Hex("#FFFFFF"), 0.22)
	AccentSoft = Fade(p.Accent, 0.08)
	AccentLine = Fade(p.Accent, 0.4)
	Highlight = Fade(p.Text, 0.05)
	if p.Light {
		AccentHigh = Mix(p.Accent, ui.Hex("#000000"), 0.08)
		AccentLine = Fade(p.Accent, 0.75)
		Highlight = ui.RGBA(255, 255, 255, 0.65)
	}
	Working, Done, Warning, Error, Idle = p.Working, p.Done, p.Warning, p.Error, p.Idle
}

// Window is the window theme MyGo's own controls draw with: the tokens of
// the current palette.
func Window() *ui.Theme {
	t := *ui.DarkTheme()
	if current.Light {
		t = *ui.LightTheme()
	}
	t.Background = Background
	t.Surface = Raised
	t.SurfaceHover = Hover
	t.SurfacePressed = Selected
	t.Border = BorderStrong
	t.Text = Text
	t.TextMuted = Muted
	t.Accent = Accent
	t.AccentHover = AccentHigh
	t.AccentPressed = Mix(Accent, Background, 0.15)
	t.AccentText = OnAccent
	t.Danger = Error
	t.Warning = Warning
	t.Success = Done
	t.Selection = Fade(Accent, 0.28)
	t.Focus = AccentLine
	t.Radius = RadiusControl
	t.Inverse, t.InverseText = Selected, Text
	t.Scrollbar = BorderStrong
	return &t
}

// Mix is a blended toward b by t, 0 to 1.
func Mix(a, b ui.Color, t float64) ui.Color {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return ui.Color{R: m(a.R, b.R), G: m(a.G, b.G), B: m(a.B, b.B), A: m(a.A, b.A)}
}

// Fade is c with alpha a, 0 to 1.
func Fade(c ui.Color, a float64) ui.Color {
	c.A = uint8(a*255 + 0.5)
	return c
}

// Hex writes c as #RRGGBB, as ui.Hex reads it back.
func Hex(c ui.Color) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }
