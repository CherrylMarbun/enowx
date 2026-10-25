package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
)

// An agent designs a canvas from the command line: a mockup built an
// element at a time, a diagram of shapes joined by an arrow, a look at it,
// its history and an export.
func TestCanvasFromTheCommandLine(t *testing.T) {
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
	t.Setenv("ENOWX_SESSION", "agent-session")
	var out bytes.Buffer
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := (runner{c: c, out: &out}).canvasCmd(args); err != nil {
			t.Fatalf("canvas %q: %v", args, err)
		}
		return out.String()
	}
	id := strings.TrimSpace(run("new", "Signup", "--frame", "desktop", "--frame", "Phone:390x700"))
	run("write", "Signup", "--html", `<style>main{padding:24px}</style><main id="main"></main>`)
	run("patch", id, "--element", "title", "--into", "main", "--html", `<h1 id="title">Join</h1>`)
	run("patch", id, "--element", "title", "--text", "Join Acme")
	run("patch", id, "--frame", "phone", "--element", "m", "--html", `<p id="m">small</p>`)
	shown := run("show", id)
	for _, want := range []string{"Frame Desktop", "1440×900", "#main <main>", "#title <h1> Join Acme", "Frame Phone", "390×700", "#m <p> small"} {
		if !strings.Contains(shown, want) {
			t.Fatalf("show lacks %q:\n%s", want, shown)
		}
	}
	if got := run("show", id, "--html"); !strings.Contains(got, `<h1 id="title">Join Acme</h1>`) {
		t.Fatalf("html:\n%s", got)
	}

	run("new", "Flow", "--kind", "diagram")
	run("shape", "Flow", "--type", "rect", "--id", "web", "--x", "40", "--y", "40", "--w", "160", "--h", "60", "--text", "Web")
	run("shape", "Flow", "--type", "ellipse", "--id", "db", "--x", "400", "--y", "40", "--text", "DB", "--fill", "#dcfce7")
	run("shape", "Flow", "--type", "arrow", "--from", "web", "--to", "db", "--text", "SQL")
	run("shape", "Flow", "--id", "web", "--y", "200") // moved, the rest kept
	shown = run("show", "Flow")
	if !strings.Contains(shown, "web rect at 40,200 160×60 \"Web\"") || !strings.Contains(shown, "arrow-3 arrow at") || !strings.Contains(shown, "web→db") {
		t.Fatalf("shapes:\n%s", shown)
	}
	dir := t.TempDir()
	png := filepath.Join(dir, "flow.png")
	run("screenshot", "Flow", png)
	if data, err := os.ReadFile(png); err != nil || !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Fatalf("screenshot: %v", err)
	}
	page := filepath.Join(dir, "signup.html")
	run("export", id, "--format", "html", "--out", page)
	if data, _ := os.ReadFile(page); !strings.Contains(string(data), "read-only") || !strings.Contains(string(data), "Join Acme") {
		t.Fatal("export html")
	}
	run("comment", "Flow", "Label the arrow", "--element", "arrow-3")
	if got := run("comments", "Flow"); !strings.Contains(got, "on #arrow-3") || !strings.Contains(got, "Label the arrow") {
		t.Fatalf("comments:\n%s", got)
	}
	hist := run("history", "Flow")
	if !strings.Contains(hist, "agent  el.set web") || !strings.Contains(hist, "*   5") {
		t.Fatalf("history:\n%s", hist)
	}
	run("undo", "Flow")
	if got := run("show", "Flow"); !strings.Contains(got, "web rect at 40,40") {
		t.Fatalf("undo:\n%s", got)
	}
	if got := run("list"); !strings.Contains(got, "diagram") || !strings.Contains(got, "Signup") {
		t.Fatalf("list:\n%s", got)
	}
}

// The canvas tools answer a screenshot as an image.
func TestCanvasScreenshotTool(t *testing.T) {
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
	if err := (runner{c: c, out: &out}).canvasCmd([]string{"new", "Board", "--kind", "board"}); err != nil {
		t.Fatal(err)
	}
	var tool mcpTool
	for _, tl := range canvasTools {
		if tl.name == "canvas_screenshot" {
			tool = tl
		}
	}
	res := runner{c: c, out: &out}.callTool(tool, runner.canvasCmd, map[string]any{"canvas": "Board", "frame": "mobile"})
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"type":"image"`) {
		t.Fatalf("tool answered %s", raw)
	}
}
