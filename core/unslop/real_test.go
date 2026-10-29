//go:build unslopreal

package unslop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Reads the designs of the canvases in UNSLOP_DIR (a canvas folder) and
// prints what it finds: go test -tags unslopreal -run Real -v ./core/unslop
func TestRealDesigns(t *testing.T) {
	dirs, _ := filepath.Glob(filepath.Join(os.Getenv("UNSLOP_DIR"), "*", "canvas.json"))
	for _, d := range dirs {
		data, _ := os.ReadFile(d)
		var c proto.Canvas
		json.Unmarshal(data, &c)
		for _, f := range c.Frames {
			if f.HTML == "" {
				continue
			}
			score, found := Scan(f.HTML, c.Kind)
			t.Logf("%s / %s\n%s", c.Title, f.Name, Report(score, found))
		}
	}
}
