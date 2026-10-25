package vaultview

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/vault"
	"github.com/enowx/enowx/ui/theme"
)

// Every brand mark kept is a platform's and draws; with SHOTS set, the
// marks of all platforms are kept as a picture.
func TestPlatformLogos(t *testing.T) {
	var ids []string
	for _, p := range vault.Platforms {
		ids = append(ids, p.ID)
	}
	for id := range brandColors {
		if !slices.Contains(ids, id) {
			t.Errorf("a mark for %q, no such platform", id)
		}
		if _, _, ok := logoOf(id); !ok {
			t.Errorf("%q's mark doesn't draw", id)
		}
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).FillWidth().FillHeight().Padding(12).Background(theme.Background).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(8).Children(func() {
				for _, p := range vault.Platforms {
					Logo(c, p, 34)
				}
			})
		})
	}, 520, 300)
	tt.Frame()
	if dir := os.Getenv("SHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "vault-logos.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, tt.Image())
	}
}
