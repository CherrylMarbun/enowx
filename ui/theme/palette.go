package theme

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Palette is one theme's colors. Every palette is dark: terminals and
// agents look best on a dark ground.
type Palette struct {
	ID, Name string

	Background, Panel, Raised, Sunken ui.Color
	Hover, Selected, Dialog           ui.Color
	Border, BorderStrong              ui.Color
	Text, TextStrong, TextSoft, Muted ui.Color
	Accent, OnAccent                  ui.Color
	Working, Done, Warning, Error     ui.Color
	Idle                              ui.Color
}

// The built-in palettes. Status colors keep their meaning in every one:
// blue works, yellow waits, green is done, red failed.
var (
	Champagne = Palette{
		ID: "champagne", Name: "Champagne",
		Background: ui.Hex("#0C0B0A"), Panel: ui.Hex("#121110"), Raised: ui.Hex("#181614"), Sunken: ui.Hex("#0A0908"),
		Hover: ui.Hex("#1D1A17"), Selected: ui.Hex("#221E1A"), Dialog: ui.Hex("#171512"),
		Border: ui.Hex("#1C1A18"), BorderStrong: ui.Hex("#2A2723"),
		Text: ui.Hex("#EEE9E2"), TextStrong: ui.Hex("#FFFFFF"), TextSoft: ui.Hex("#BDB5AA"), Muted: ui.Hex("#8A8378"),
		Accent: ui.Hex("#D8C3A0"), OnAccent: ui.Hex("#1A150E"),
		Working: ui.Hex("#9DB6F5"), Done: ui.Hex("#8FCFA5"), Warning: ui.Hex("#E3C07A"), Error: ui.Hex("#E58A84"), Idle: ui.Hex("#6E675E"),
	}
	Obsidian = Palette{
		ID: "obsidian", Name: "Obsidian Silver",
		Background: ui.Hex("#0B0C0E"), Panel: ui.Hex("#111215"), Raised: ui.Hex("#16181B"), Sunken: ui.Hex("#08090A"),
		Hover: ui.Hex("#1B1D21"), Selected: ui.Hex("#22252A"), Dialog: ui.Hex("#17191C"),
		Border: ui.Hex("#1B1D20"), BorderStrong: ui.Hex("#292C31"),
		Text: ui.Hex("#F2F3F5"), TextStrong: ui.Hex("#FFFFFF"), TextSoft: ui.Hex("#B4B8C0"), Muted: ui.Hex("#80858E"),
		Accent: ui.Hex("#E2E4E9"), OnAccent: ui.Hex("#0B0C0E"),
		Working: ui.Hex("#9DB6F5"), Done: ui.Hex("#86D0A8"), Warning: ui.Hex("#E6C47E"), Error: ui.Hex("#EE8C8C"), Idle: ui.Hex("#5E636C"),
	}
	Midnight = Palette{
		ID: "midnight", Name: "Midnight",
		Background: ui.Hex("#0A0C11"), Panel: ui.Hex("#0F1218"), Raised: ui.Hex("#141821"), Sunken: ui.Hex("#07090D"),
		Hover: ui.Hex("#181D28"), Selected: ui.Hex("#1C2333"), Dialog: ui.Hex("#121621"),
		Border: ui.Hex("#161B25"), BorderStrong: ui.Hex("#232A38"),
		Text: ui.Hex("#EAEEF6"), TextStrong: ui.Hex("#FFFFFF"), TextSoft: ui.Hex("#B3BBCB"), Muted: ui.Hex("#7F889A"),
		Accent: ui.Hex("#9DB6F5"), OnAccent: ui.Hex("#0A0F1E"),
		Working: ui.Hex("#9DB6F5"), Done: ui.Hex("#7FD3A6"), Warning: ui.Hex("#EFC87C"), Error: ui.Hex("#F08C92"), Idle: ui.Hex("#5C6577"),
	}
	Copper = Palette{
		ID: "copper", Name: "Copper",
		Background: ui.Hex("#0E0F11"), Panel: ui.Hex("#131417"), Raised: ui.Hex("#18191D"), Sunken: ui.Hex("#0A0B0C"),
		Hover: ui.Hex("#1C1E22"), Selected: ui.Hex("#222429"), Dialog: ui.Hex("#1A1B1F"),
		Border: ui.Hex("#1B1C1F"), BorderStrong: ui.Hex("#2A2B30"),
		Text: ui.Hex("#ECEAE7"), TextStrong: ui.Hex("#FFFFFF"), TextSoft: ui.Hex("#B7B3AD"), Muted: ui.Hex("#868279"),
		Accent: ui.Hex("#D49A6A"), OnAccent: ui.Hex("#1A0F07"),
		Working: ui.Hex("#9DB6F5"), Done: ui.Hex("#8FCFA5"), Warning: ui.Hex("#E3C07A"), Error: ui.Hex("#E88B85"), Idle: ui.Hex("#5F5C57"),
	}
	Graphite = Palette{
		ID: "graphite", Name: "Graphite",
		Background: ui.Hex("#0E0F12"), Panel: ui.Hex("#121418"), Raised: ui.Hex("#181B21"), Sunken: ui.Hex("#0B0C0F"),
		Hover: ui.Hex("#1A1E26"), Selected: ui.Hex("#1E2330"), Dialog: ui.Hex("#16181D"),
		Border: ui.Hex("#1F2228"), BorderStrong: ui.Hex("#2A2E36"),
		Text: ui.Hex("#E6E7EA"), TextStrong: ui.Hex("#FFFFFF"), TextSoft: ui.Hex("#B8BCC6"), Muted: ui.Hex("#8B909A"),
		Accent: ui.Hex("#7FA6FF"), OnAccent: ui.Hex("#0E0F12"),
		Working: ui.Hex("#7FA6FF"), Done: ui.Hex("#5FD39A"), Warning: ui.Hex("#F2C15B"), Error: ui.Hex("#FF7A80"), Idle: ui.Hex("#6B7080"),
	}
)

// Palettes is every built-in palette, the default first.
var Palettes = []Palette{Champagne, Obsidian, Midnight, Copper, Graphite}

// Accents are the colors a user can give any palette instead of its own.
var Accents = []ui.Color{
	ui.Hex("#D8C3A0"), // champagne
	ui.Hex("#E2E4E9"), // silver
	ui.Hex("#9DB6F5"), // blue
	ui.Hex("#B4A4F2"), // violet
	ui.Hex("#E9A6B8"), // rose
	ui.Hex("#D49A6A"), // copper
	ui.Hex("#E3C07A"), // amber
	ui.Hex("#8FCFA5"), // sage
	ui.Hex("#7CCBD3"), // teal
}

// Lookup is the palette with the given ID, else the default; a non-zero
// accent replaces the palette's own.
func Lookup(id string, accent ui.Color) Palette {
	p := Palettes[0]
	for _, q := range Palettes {
		if q.ID == id {
			p = q
		}
	}
	if accent.A != 0 {
		p.Accent = accent
		p.OnAccent = Mix(p.Background, ui.Hex("#000000"), 0.3)
	}
	return p
}

// Terminal is the terminals' colors for the current palette: its ground
// and text, with ANSI colors tuned to it.
func Terminal() *terminal.Theme {
	t := terminal.DarkTheme()
	fg := Mix(Text, Background, 0.08)
	t.Foreground, t.Background = fg, Background
	t.Cursor = Accent
	t.Selection = Fade(Accent, 0.25)
	t.Palette = [16]ui.Color{
		BorderStrong, Error, Done, Warning,
		Working, ui.Hex("#C7A4E8"), ui.Hex("#7CCBD3"), fg,
		Idle, Mix(Error, TextStrong, 0.2), Mix(Done, TextStrong, 0.2), Mix(Warning, TextStrong, 0.2),
		Mix(Working, TextStrong, 0.2), ui.Hex("#DAB8F0"), ui.Hex("#9DDCE3"), TextStrong,
	}
	return t
}
