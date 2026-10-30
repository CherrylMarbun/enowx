package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
)

// A canvas agent keeps an SVG it drew as the canvas's picture; what isn't
// an SVG, or a URL that isn't https, is refused.
func TestCanvasImage(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a, b := net.Pipe()
	go srv.ServeConn(a)
	c, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var out bytes.Buffer
	r := runner{c: c, out: &out}
	if err := r.canvasCmd([]string{"new", "Ledger"}); err != nil {
		t.Fatal(err)
	}
	var tool mcpTool
	for _, tl := range canvasTools {
		if tl.name == "canvas_image" {
			tool = tl
		}
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#c2410c"/></svg>`
	res := r.callTool(tool, runner.canvasCmd, map[string]any{"canvas": "Ledger", "name": "hero", "svg": svg})
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), "assets/hero.svg") {
		t.Fatalf("kept: %s", raw)
	}
	cv, err := r.findCanvas("Ledger")
	if err != nil {
		t.Fatal(err)
	}
	if data, err := c.CanvasAsset(cv.ID, "hero.svg"); err != nil || string(data) != svg {
		t.Fatalf("the asset: %q %v", data, err)
	}
	if err := r.canvasCmd([]string{"image", "Ledger", "--name", "x", "--svg", "<div/>"}); err == nil {
		t.Fatal("not an SVG was kept")
	}
	if err := r.canvasCmd([]string{"image", "Ledger", "--name", "x", "--url", "http://example.com/a.png"}); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("a plain http URL: %v", err)
	}
}
