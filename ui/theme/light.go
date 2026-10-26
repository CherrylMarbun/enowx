package theme

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// LookupAppearance returns a palette with surfaces and contrast composed
// for the requested appearance. The palette ID and chosen accent survive.
func LookupAppearance(id string, accent ui.Color, light bool) Palette {
	if !light {
		return Lookup(id, accent)
	}
	dark := Lookup(id, ui.Color{})
	p := Palette{
		ID: dark.ID, Name: dark.Name, Light: true,
		Background: ui.Hex("#F4F6F9"), Panel: ui.Hex("#FFFFFF"), Raised: ui.Hex("#FFFFFF"), Sunken: ui.Hex("#EDF0F5"),
		Hover: ui.Hex("#EAF0F8"), Selected: ui.Hex("#E0E8F3"), Dialog: ui.Hex("#FFFFFF"),
		Border: ui.Hex("#D9DFE8"), BorderStrong: ui.Hex("#778497"),
		Text: ui.Hex("#243043"), TextStrong: ui.Hex("#101828"), TextSoft: ui.Hex("#475467"), Muted: ui.Hex("#536176"),
		Accent: ui.Hex("#315FBD"), Working: ui.Hex("#245BC1"), Done: ui.Hex("#166B42"), Warning: ui.Hex("#855300"), Error: ui.Hex("#B42318"), Idle: ui.Hex("#6A7688"),
	}
	switch p.ID {
	case Champagne.ID:
		p.Background, p.Sunken = ui.Hex("#F8F6F1"), ui.Hex("#F0ECE4")
		p.Hover, p.Selected = ui.Hex("#F1ECE3"), ui.Hex("#E9E0D1")
		p.Border, p.BorderStrong = ui.Hex("#E2DACB"), ui.Hex("#8E7C63")
		p.Text, p.TextStrong, p.TextSoft, p.Muted = ui.Hex("#322B22"), ui.Hex("#20190F"), ui.Hex("#60523F"), ui.Hex("#6A5B47")
		p.Accent = ui.Hex("#826235")
	case Obsidian.ID:
		p.Accent = ui.Hex("#4E5A6B")
	case Midnight.ID:
		p.Accent = ui.Hex("#355CAD")
	case Copper.ID:
		p.Background, p.Sunken = ui.Hex("#F8F5F2"), ui.Hex("#F0E9E3")
		p.Hover, p.Selected = ui.Hex("#F3EAE1"), ui.Hex("#ECDDCE")
		p.Accent = ui.Hex("#9C5429")
	}
	if accent.A != 0 {
		p.Accent = accent
	}
	// The dark palettes use pale accents. Keep their hue in light mode,
	// with enough depth for selected labels and links on white surfaces.
	for contrast(p.Accent, p.Sunken) < 4.5 || contrast(p.Accent, p.Panel) < 4.5 || contrast(p.Accent, p.Selected) < 4.5 {
		p.Accent = Mix(p.Accent, ui.Hex("#000000"), 0.06)
	}
	p.OnAccent = contrastInk(p.Accent)
	return p
}

func contrastInk(background ui.Color) ui.Color {
	dark, light := ui.Hex("#101828"), ui.Hex("#FFFFFF")
	if contrast(dark, background) >= contrast(light, background) {
		return dark
	}
	return light
}

func contrast(a, b ui.Color) float64 {
	luminance := func(c ui.Color) float64 {
		linear := func(v uint8) float64 {
			x := float64(v) / 255
			if x <= 0.04045 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
	}
	x, y := luminance(a), luminance(b)
	return (max(x, y) + 0.05) / (min(x, y) + 0.05)
}
