package icon

import (
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/iconlib"
)

var update = flag.Bool("update", false, "record the icons' drawing as it is now")

// The icons are drawn by the app's own renderer, so macOS, Linux and
// Windows draw the same: icons of every library, in colors and sizes,
// match the drawing recorded, but for edges smoothed a little otherwise.
// With SHOTS set the picture is kept.
func TestIconsDrawTheSameOnEverySystem(t *testing.T) {
	refs := []string{iconlib.Default}
	for _, lib := range []string{"lucide", "tabler", "phosphor"} {
		for _, ic := range iconlib.Search("rocket", []string{lib}, 4) {
			refs = append(refs, ic.Ref())
		}
	}
	refs = append(refs, "lucide:rocket#e11d48", "tabler:star#16a34a@48", "phosphor:heart#2563eb@96")
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Width(640).Height(200).Wrap().Gap(8).Padding(8).Background(ui.Hex("#ffffff")).Children(func() {
			for i, ref := range refs {
				svg, color, colored := Note(ref)
				if svg == nil {
					t.Fatalf("%s doesn't draw", ref)
				}
				if !colored {
					color = ui.Hex("#0f172a")
				}
				size := float32(16 + 8*(i%4))
				if r, err := iconlib.Parse(ref); err == nil && r.Size > 0 {
					size = float32(r.Size)
				}
				ui.Icon(c.Key(ref), svg).FontSize(size).TextColor(color)
			}
		})
	}, 640, 200)
	tt.Frame()
	img := tt.Image()
	if dir := os.Getenv("SHOTS"); dir != "" {
		if f, err := os.Create(filepath.Join(dir, "icons-drawn.png")); err == nil {
			_ = png.Encode(f, img)
			f.Close()
		}
	}
	golden := filepath.Join("testdata", "icons-drawn.png")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(golden)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(golden)
	if err != nil {
		t.Fatalf("no drawing recorded: go test ./ui/icon -update (%v)", err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	// The same shapes, colors and sizes: a system's own smoothing of edges
	// may differ a little, a different drawing more.
	if want.Bounds() != img.Bounds() {
		t.Fatalf("drawn %v, recorded %v", img.Bounds(), want.Bounds())
	}
	off, inked := 0, 0
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			r1, g1, b1, _ := img.At(x, y).RGBA()
			r2, g2, b2, _ := want.At(x, y).RGBA()
			if r2 < 0xf000 || g2 < 0xf000 || b2 < 0xf000 {
				inked++
			}
			d := max(diff(r1, r2), diff(g1, g2), diff(b1, b2)) >> 8
			if d > 64 {
				off++
			}
		}
	}
	if inked == 0 || float64(off) > 0.05*float64(inked) {
		t.Fatalf("the icons draw otherwise here than recorded: %d of %d inked pixels differ", off, inked)
	}
}

func diff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
