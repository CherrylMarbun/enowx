package ui

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
)

// floatingTester starts the panel on a test app, up, and renders it once.
func floatingTester(t *testing.T, app *App) *ui.Tester {
	t.Helper()
	app.floatingShown = true
	tt := ui.NewTester(app.floatingView, floatingSize, floatingSize)
	tt.Frame()
	return tt
}

// floatingCenter is the middle of the box of s, or a failure.
func floatingCenter(t *testing.T, tt *ui.Tester, s string) (x, y float32) {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("no %q on the panel: %q", s, tt.Texts())
	}
	return r.X + r.W/2, r.Y + r.H/2
}

// floatingWithSessions is an app with sessions on this PC and on a VPS, and
// a tester on its panel.
func floatingWithSessions(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	app := New(Options{})
	app.host = LocalHost()
	app.sessions = []proto.SessionInfo{
		{ID: "idle", Name: "Idle one", Created: time.Now().Add(-3 * time.Hour)},
		{ID: "work", Name: "Working one", State: proto.StateWorking, Created: time.Now().Add(-2 * time.Hour)},
		{ID: "wait", Name: "Waiting one", State: proto.StateWaiting, Created: time.Now().Add(-time.Hour)},
	}
	vps := hosts.Host{ID: "vps", Name: "lianbot", Address: "root@10.0.0.1"}
	app.hostList = []hosts.Host{vps}
	app.watches = map[string]*hostWatch{"vps": {host: vps, status: hostStatus{
		state: hostOnline,
		sessions: []proto.SessionInfo{
			{ID: "v1", Name: "Server job", Agent: "claude", State: proto.StateWorking},
		},
	}}}
	return app, floatingTester(t, app)
}

// The floating control rings seven actions around the centre, in a pie cut
// into slices: all of them usable without a connection except those that
// need the host or a project. The first is Active sessions, which holds the
// two sources.
func TestFloatingControlActions(t *testing.T) {
	app := New(Options{})
	tt := floatingTester(t, app)
	ring := app.floatingActions()
	if len(ring) != floatingWedges {
		t.Fatalf("the pie holds %d slices, want %d", len(ring), floatingWedges)
	}
	if ring[0].label != "Active sessions" {
		t.Fatalf("the first slice is %q, want Active sessions", ring[0].label)
	}
	for _, want := range []string{
		"Active sessions", "New session", "Open folder", "Panels",
		"Command palette", "Settings", "Appearance",
	} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	// The fans' children are off the pie until their wedge opens.
	for _, act := range append(app.floatingSources(), app.floatingPanels()...) {
		if _, ok := tt.Find(act.label); ok {
			t.Fatalf("%q shows though no fan is open", act.label)
		}
	}
	if _, ok := tt.Find("Hide the floating control"); !ok {
		t.Fatal("no hub in the middle")
	}
	// Each slice's button sits on its own slice, in its own wedge: the
	// angle between two of them is a seventh of the circle.
	seen := map[ui.Rect]bool{}
	centres := map[string][2]float32{}
	for i, act := range ring {
		r, ok := tt.Find(act.label)
		if !ok {
			t.Fatalf("no %q on the pie", act.label)
		}
		if seen[r] {
			t.Fatalf("%q sits on another slice", act.label)
		}
		seen[r] = true
		if r.X < 0 || r.Y < 0 || r.X+r.W > floatingSize || r.Y+r.H > floatingSize {
			t.Fatalf("%q hangs off the panel: %+v", act.label, r)
		}
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		centres[act.key] = [2]float32{cx, cy}
		dx, dy := float64(cx-floatingSize/2), float64(cy-floatingSize/2)
		if d := math.Hypot(dx, dy); math.Abs(d-floatingIconAt) > 2 {
			t.Fatalf("%q is %0.f DIPs from the middle, want %d", act.label, d, floatingIconAt)
		}
		// The angle is the slice's own: a seventh of the circle each, from
		// the top, clockwise.
		angle := math.Atan2(-dy, dx)
		want := math.Pi/2 - float64(i)*floatingStep
		if d := math.Abs(floatingWrap(angle - want)); d > 0.02 {
			t.Fatalf("%q sits %.3f rad off its slice", act.label, d)
		}
	}
	// No two slices' buttons touch: seven wedges leave room between them.
	for a := range centres {
		for b := range centres {
			if a >= b {
				continue
			}
			ax, ay := centres[a][0], centres[a][1]
			bx, by := centres[b][0], centres[b][1]
			if d := math.Hypot(float64(ax-bx), float64(ay-by)); d < floatingSliceTarget {
				t.Fatalf("%q and %q sit %0.f DIPs apart", a, b, d)
			}
		}
	}
	enabled := func(acts []floatingAction, key string) bool {
		for _, act := range acts {
			if act.key == key {
				return act.enabled
			}
		}
		t.Fatalf("no action %q", key)
		return false
	}
	if enabled(ring, "new") || enabled(ring, "open") || enabled(ring, "settings") {
		t.Fatal("actions needing the host are enabled with no connection")
	}
	if !enabled(ring, "palette") || !enabled(ring, "appearance") {
		t.Fatal("an action of the app itself is disabled")
	}
	if !enabled(ring, floatingActiveFan) || !enabled(ring, floatingPanelsFan) {
		t.Fatal("a fan's own wedge is disabled")
	}
	// With a project, the split view comes on; New session still needs the
	// host it would ask.
	app.projects = []proto.ProjectInfo{{Path: "/p/api", Name: "api"}}
	app.project = "/p/api"
	if !enabled(app.floatingPanels(), "split") {
		t.Fatal("the split view stays disabled with a project")
	}
	if enabled(ring, "new") || enabled(ring, "open") {
		t.Fatal("actions needing the host are enabled with no connection")
	}
}

// A surviving shell tab keeps its parent session recognizable, while
// navigation and other agents' tabs retain their original identity.
func TestFloatingShellTabKeepsParentIdentity(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(strconv.FormatBool(remote), func(t *testing.T) {
			app := New(Options{})
			parent := proto.SessionInfo{ID: "omp-parent", Agent: "omp", Name: "OMP session", Exited: true}
			shell := proto.SessionInfo{ID: "shell-tab", Parent: parent.ID, Agent: "shell"}
			if remote {
				app.watches = map[string]*hostWatch{"vps": {status: hostStatus{sessions: []proto.SessionInfo{parent, shell}}}}
			} else {
				app.sessions = []proto.SessionInfo{parent, shell}
			}
			acts := app.floatingSessionWedges([]proto.SessionInfo{shell})
			if acts[0].svg != icon.Agent("omp") || acts[0].label != parent.Name {
				t.Fatal("a live shell tab lost the stopped OMP parent's identity")
			}
			if !remote {
				acts[0].run()
				if app.selected != parent.ID || app.tabs[parent.ID] != shell.ID {
					t.Fatal("the inherited icon did not open the original shell tab")
				}
			}
			agentTab := shell
			agentTab.Agent = "codex"
			if got := app.floatingSessionWedges([]proto.SessionInfo{agentTab}); got[0].svg != icon.Agent("codex") {
				t.Fatal("a different agent tab inherited the parent's icon")
			}
		})
	}
}

// Direct agent commands keep their icons even when created as shell or
// custom sessions, including commands from a host with different paths.
func TestFloatingSessionCommandIcons(t *testing.T) {
	app := New(Options{})
	for _, tc := range []struct {
		name    string
		session proto.SessionInfo
		want    *ui.SVG
	}{
		{"OMP preset", proto.SessionInfo{Agent: "omp"}, icon.Agent("omp")},
		{"OMP command", proto.SessionInfo{Command: []string{"omp"}}, icon.Agent("omp")},
		{"Antigravity preset", proto.SessionInfo{Agent: "antigravity"}, icon.Agent("antigravity")},
		{"Cline preset", proto.SessionInfo{Agent: "cline"}, icon.Agent("cline")},
		{"Hermes preset", proto.SessionInfo{Agent: "hermes"}, icon.Agent("hermes")},
		{"Windows Codex command", proto.SessionInfo{Agent: "shell", Command: []string{`C:\Tools\codex.cmd`}}, icon.Agent("codex")},
		{"remote Claude command", proto.SessionInfo{Agent: "custom", Command: []string{"/usr/local/bin/claude"}}, icon.Agent("claude")},
		{"explicit preset", proto.SessionInfo{Agent: "gemini", Command: []string{"launcher"}}, icon.Agent("gemini")},
		{"shell", proto.SessionInfo{Agent: "shell", Command: []string{"pwsh.exe"}}, icon.Terminal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := app.floatingSessionWedges([]proto.SessionInfo{tc.session})
			if len(acts) != 1 || acts[0].svg != tc.want {
				t.Fatal("session ring does not use the project's agent icon")
			}
			if acts[0].svg == icon.Bot {
				t.Fatal("a supported agent still uses the generic bot icon")
			}
		})
	}
}

// chart's own slices are what the pie is made of: this guards the import
// against a silent swap to an approximation.
var _ = chart.Pie
