package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLightPalettesKeepReadableSurfacesAndStatus(t *testing.T) {
	old := Current()
	defer Apply(old)
	accents := append([]ui.Color{{}}, Accents...)
	accents = append(accents, ui.Hex("#FFFFFF"), ui.Hex("#000000"), ui.Hex("#FF0000"))
	for _, preset := range Palettes {
		for _, accent := range accents {
			p := LookupAppearance(preset.ID, accent, true)
			Apply(p)
			if Window().Dark || p.ID != preset.ID {
				t.Fatalf("%s did not keep its palette in light appearance", preset.ID)
			}
			for _, bg := range []ui.Color{p.Background, p.Panel, p.Raised, p.Sunken} {
				if contrast(p.BorderStrong, bg) < 3 {
					t.Fatalf("%s: control outlines lost contrast", preset.ID)
				}
				for _, fg := range []ui.Color{p.Text, p.TextStrong, p.TextSoft, p.Muted, p.Accent, p.Working, p.Done, p.Warning, p.Error} {
					if ratio := contrast(fg, bg); ratio < 4.5 {
						t.Fatalf("%s: %s on %s contrast %.2f", preset.ID, Hex(fg), Hex(bg), ratio)
					}
				}
			}
			if contrast(p.Accent, p.Selected) < 4.5 || contrast(p.OnAccent, AccentHigh) < 4.5 || contrast(p.OnAccent, p.Accent) < 4.5 {
				t.Fatalf("%s: selected/primary labels lost contrast", preset.ID)
			}
			terminal := Terminal()
			if terminal.Background != p.Background || contrast(terminal.Foreground, terminal.Background) < 4.5 {
				t.Fatal("terminal did not follow light appearance")
			}
			if int(Ground.R)+int(Ground.G)+int(Ground.B) < 600 {
				t.Fatal("light mode still has a dark app ground")
			}
		}
	}
}

func TestSwitchingBackToDarkRestoresPalette(t *testing.T) {
	old := Current()
	defer Apply(old)
	for _, preset := range Palettes {
		Apply(LookupAppearance(preset.ID, ui.Color{}, true))
		Apply(LookupAppearance(preset.ID, ui.Color{}, false))
		if Current() != preset || !Window().Dark {
			t.Fatalf("%s did not restore its original dark palette", preset.ID)
		}
	}
}
