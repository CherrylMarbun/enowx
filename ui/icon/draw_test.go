package icon

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/iconlib"
)

var update = flag.Bool("update", false, "record the icons' drawing as it is now")

// The icons are drawn by the same code everywhere (no system's renderer),
// so macOS, Linux and Windows draw the same pixels: the drawing of icons of
// every library, in colors and sizes, matches the one recorded. With SHOTS
// set the picture is kept.
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
	sum := sha256.Sum256(img.Pix)
	got := hex.EncodeToString(sum[:])
	golden := filepath.Join("testdata", "icons-drawn.sha256")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("no drawing recorded: go test ./ui/icon -update (%v)", err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("the icons draw otherwise here than recorded: %s, recorded %s", got, strings.TrimSpace(string(want)))
	}
}
