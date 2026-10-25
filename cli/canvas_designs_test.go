package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An agent asked for several designs makes them in one canvas, describes
// them, copies one as a variant, and has the app show and compare them.
func TestCanvasDesignsFromTheCommandLine(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	app := make(chan proto.CanvasChange, 8)
	a, b := net.Pipe()
	go srv.ServeConn(a)
	c, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	a2, b2 := net.Pipe()
	go srv.ServeConn(a2)
	watcher, err := client.New(b2, func(ev proto.Event) {
		var ch proto.CanvasChange
		if ev.Name == proto.EventCanvasChanged && json.Unmarshal(ev.Data, &ch) == nil && (ch.Op == proto.CanvasOpen || ch.Op == proto.CanvasCompare) {
			app <- ch
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	var out bytes.Buffer
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := (runner{c: c, out: &out}).canvasCmd(args); err != nil {
			t.Fatalf("canvas %q: %v", args, err)
		}
		return out.String()
	}
	id := strings.TrimSpace(run("new", "Landing", "--frame", "Hero A"))
	run("design", id, "1", "--description", "Bold type", "--page", "Home", "--variant", "A")
	if got := run("new-design", id, "--title", "Hero B", "--page", "Home", "--variant", "B"); !strings.HasPrefix(got, "Design 2: Hero B") {
		t.Fatalf("new-design: %q", got)
	}
	run("patch", id, "--frame", "2", "--element", "cta", "--html", `<button id="cta">Go</button>`)
	if got := run("new-design", id, "--from", "2", "--variant", "C"); !strings.HasPrefix(got, "Design 3: Hero B (2)") {
		t.Fatalf("copy: %q", got)
	}
	run("favorite", id, "3")
	run("pick", id, "design-3")
	run("move-design", id, "3", "--before", "1")
	list := run("designs", id)
	for _, want := range []string{"3 designs", "1  Hero B (2)  (Home × C)", "[winner] [favorite]", "canvas://" + id + "/design-1", "Bold type"} {
		if !strings.Contains(list, want) {
			t.Fatalf("designs lack %q:\n%s", want, list)
		}
	}
	run("goto", id, "2")
	cmp := run("compare", id, "1", "2")
	if !strings.Contains(cmp, "Hero B (2) → Hero A: 1 differences") || !strings.Contains(cmp, "removed #cta") {
		t.Fatalf("compare:\n%s", cmp)
	}
	for _, want := range []string{proto.CanvasOpen, proto.CanvasCompare} {
		select {
		case ch := <-app:
			if ch.Op != want || want == proto.CanvasOpen && ch.Frame != ch.Canvas.Frames[1].ID || want == proto.CanvasCompare && len(ch.Frames) != 2 {
				t.Fatalf("app told %+v", ch)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("the app wasn't told %s", want)
		}
	}
	run("rm-design", id, "3")
	if got := run("designs", id); !strings.Contains(got, "2 designs") {
		t.Fatalf("after rm:\n%s", got)
	}
}
