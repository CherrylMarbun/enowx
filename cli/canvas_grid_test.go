package cli

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
)

// An agent gives a design a grid, lays its HTML out on it, and puts an
// icon in a mockup.
func TestCanvasGridFromTheCommandLine(t *testing.T) {
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
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := (runner{c: c, out: &out}).canvasCmd(args); err != nil {
			t.Fatalf("canvas %q: %v", args, err)
		}
		return out.String()
	}
	id := strings.TrimSpace(run("new", "Home"))
	if got := run("grid", id); !strings.Contains(got, "has no grid") {
		t.Fatalf("no grid: %q", got)
	}
	got := run("grid", id, "--cols", "12", "--rows", "4", "--gap", "24", "--margin", "48", "--area", "header:1,1,12,1", "--area", "main:1,2,8,3", "--area", "side:9,2,4,3", "--apply")
	for _, want := range []string{"12 columns of 90, 4 rows of", "#side: columns 9–12, rows 2–4", "#main { grid-area: 2 / 1 / 5 / 9; }", "fill each area"} {
		if !strings.Contains(got, want) {
			t.Fatalf("grid lacks %q:\n%s", want, got)
		}
	}
	if shown := run("show", id); !strings.Contains(shown, "#header <section>") || !strings.Contains(shown, "#side <section>") {
		t.Fatalf("not laid out:\n%s", shown)
	}
	if svg := run("icon", "lucide:rocket", "--size", "20"); !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, `width="20" height="20"`) || strings.Count(svg, ` width="`) != 1 {
		t.Fatalf("icon: %s", svg)
	}
}
