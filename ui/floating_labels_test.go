package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// A shell's path is never the pie's name for it: the terminal title of a plain
// shell is its program, and that filled the hub with a broken path.
func TestFloatingHubTitleFits(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	app.sessions = []proto.SessionInfo{{
		ID: "shell", Title: `C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe`,
		Created: time.Now(),
	}}
	tt := floatingTester(t, app)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	for _, s := range tt.Texts() {
		if floatingPathLike(s) {
			t.Fatalf("the pie shows a path: %q", s)
		}
	}
	if !tt.HasText("Session") {
		t.Fatalf("the shell falls back to no name: %q", tt.Texts())
	}
	// The name the pie gives it is the fallback, not the title.
	if got := app.floatingSessionName(app.sessions[0]); got != "Session" {
		t.Fatalf("the pie calls the shell %q, want Session", got)
	}
	// A user-given name still wins over the fallback.
	if got := app.floatingSessionName(proto.SessionInfo{Name: "deploy", Title: `C:\x\y.exe`}); got != "deploy" {
		t.Fatalf("a named session is %q, want deploy", got)
	}
	// And a real title, when it says something, is kept.
	if got := app.floatingSessionName(proto.SessionInfo{Title: "fix the tests"}); got != "fix the tests" {
		t.Fatalf("a titled session is %q", got)
	}
}

// A long session name is clipped to what the hub holds, and the clipped title
// is the only text that can be the hub's, so its box proves the fit.
func TestFloatingHubClipsLongNames(t *testing.T) {
	const name = "a very long session name that cannot fit any circle"
	app := New(Options{})
	app.host = LocalHost()
	app.sessions = []proto.SessionInfo{{ID: "long", Name: name, Created: time.Now()}}
	tt := floatingTester(t, app)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	// Hover the slice, so the hub names it.
	x, y := floatingCenter(t, tt, name)
	tt.Move(x, y)
	tt.Advance(time.Second)
	// The hub's title is the clipped name: shorter than the slice's label,
	// so it is the only text that reads that way.
	want := floatingClip(name, floatingHubBudget*floatingHubLines)
	if !strings.HasSuffix(want, "…") {
		t.Fatalf("the clip kept %q whole", want)
	}
	r, ok := tt.Find(want)
	if !ok {
		t.Fatalf("the hub shows no clipped title %q: %q", want, tt.Texts())
	}
	cx, cy := float32(floatingSize)/2, float32(floatingSize)/2
	if r.X < cx-floatingHubSize/2 || r.Y < cy-floatingHubSize/2 ||
		r.X+r.W > cx+floatingHubSize/2 || r.Y+r.H > cy+floatingHubSize/2 {
		t.Fatalf("the hub's title spills out of the %d DIP circle: %+v", floatingHubSize, r)
	}
}

// A fan child names itself in the centre circle, as a ring slice does, and
// carries no bubble of its own: with a child hovered exactly one bubble-sized
// text is up, the hub's, and the ring's own bubbles are absent, so nothing can
// collide over the pie.
func TestFloatingFanNamesInTheHub(t *testing.T) {
	app := New(Options{})
	tt := floatingTester(t, app)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	// Off the pie: the click left the pointer on the wedge, and the wedge's
	// sub-slices now bloom under it.
	tt.Move(2, 2)
	tt.Advance(floatingBloomTime)
	// With nothing hovered the hub shows its mark. Each child has an
	// accessible button label and a visible caption.
	before := tt.Texts()
	for _, name := range []string{"This PC", "VPS"} {
		if got := floatingTextCount(before, name); got != 2 {
			t.Fatalf("%q is up %d times before it is hovered, want its button and caption: %q", name, got, before)
		}
	}
	// Hovering a child adds the hub's name and nothing else: no bubble.
	x, y := floatingCenter(t, tt, "VPS")
	tt.Move(x, y)
	tt.Advance(floatingBloomTime)
	texts := tt.Texts()
	if got, want := len(texts), len(before)+1; got != want {
		t.Fatalf("hovering a child shows %d texts, want %d (one hub name): %q", got, want, texts)
	}
	if got := floatingTextCount(texts, "VPS"); got != 3 {
		t.Fatalf("%q is up %d times, want the child's label, caption and hub name: %q", "VPS", got, texts)
	}
	if got := floatingTextCount(texts, "This PC"); got != 2 {
		t.Fatalf("the unhovered child's name is up %d times, want its label and caption: %q", got, texts)
	}
	// The ring's own bubbles are absent: a slice's *short* name (its bubble)
	// is not up, only its label, on its button. "Palette" is the one short
	// name no label contains, so it can only come from a bubble.
	if floatingTextCount(texts, "Palette") != 0 {
		t.Fatalf("a ring bubble is up with the fan open: %q", texts)
	}
	for _, act := range app.floatingActions() {
		if act.key == floatingActiveFan || act.short == act.label {
			continue
		}
		if floatingTextCount(texts, act.short) != 0 {
			t.Fatalf("the ring's %q bubble is up with the fan open: %q", act.short, texts)
		}
	}
	// A bubble is a pill with a border and a shadow, wider than the hub; the
	// hub is a circle. So the only text that could be a bubble would
	// be wider than floatingHubSize, and none is.
	for _, s := range texts {
		r, ok := tt.Find(s)
		if !ok {
			continue
		}
		if r.W > floatingHubSize && r.H > floatingHubSize {
			t.Fatalf("%q is %+v, a bubble rather than the hub", s, r)
		}
	}
}

// floatingTextCount is how many of texts read exactly s.
func floatingTextCount(texts []string, s string) int {
	n := 0
	for _, t := range texts {
		if t == s {
			n++
		}
	}
	return n
}

// Hovering a slice puts its name in the hub and nowhere else: no pill floats
// out past the rim. "Palette" is the short name of Command palette, which its
// label does not contain, so any "Palette" can only be the hub's.
func TestFloatingHoverShowsName(t *testing.T) {
	app := New(Options{})
	tt := floatingTester(t, app)
	before := len(tt.Texts())
	x, y := floatingCenter(t, tt, "Command palette")
	tt.Move(x, y)
	tt.Advance(floatingBloomTime)
	if !tt.HasText("Palette") {
		t.Fatalf("the hub names no hovered slice: %q", tt.Texts())
	}
	// A hover adds exactly one text — the hub's — and no bubble with it.
	if got, want := len(tt.Texts()), before+1; got != want {
		t.Fatalf("hovering shows %d texts, want %d: %q", got, want, tt.Texts())
	}
	if got := floatingTextCount(tt.Texts(), "Palette"); got != 1 {
		t.Fatalf("%q is up %d times, want only the hub's once: %q", "Palette", got, tt.Texts())
	}
	// And it is inside the hub, not out past the rim.
	r, ok := tt.Find("Palette")
	if !ok {
		t.Fatalf("no Palette: %q", tt.Texts())
	}
	cx, cy := float32(floatingSize)/2, float32(floatingSize)/2
	if r.X < cx-floatingHubSize/2 || r.Y < cy-floatingHubSize/2 ||
		r.X+r.W > cx+floatingHubSize/2 || r.Y+r.H > cy+floatingHubSize/2 {
		t.Fatalf("the hovered name floats outside the hub: %+v", r)
	}
	tt.Move(2, 2)
	tt.Advance(time.Second)
	if tt.HasText("Palette") {
		t.Fatalf("a name stayed behind: %q", tt.Texts())
	}
	if got := len(tt.Texts()); got != before {
		t.Fatalf("%d texts after the pointer left, want %d: %q", got, before, tt.Texts())
	}
}

// Nothing floats past the rim: with a slice hovered, the only text outside the
// hub is a slice's own label on its own button, and no text is a pill.
func TestFloatingNothingFloats(t *testing.T) {
	app := New(Options{})
	tt := floatingTester(t, app)
	cx, cy := float32(floatingSize)/2, float32(floatingSize)/2
	inHub := func(r ui.Rect) bool {
		return r.X >= cx-floatingHubSize/2 && r.Y >= cy-floatingHubSize/2 &&
			r.X+r.W <= cx+floatingHubSize/2 && r.Y+r.H <= cy+floatingHubSize/2
	}
	// Every slice's own button is floatingSliceTarget across: a bubble was
	// wider than that and taller than a line.
	for _, act := range app.floatingActions() {
		x, y := floatingCenter(t, tt, act.label)
		tt.Move(x, y)
		tt.Advance(floatingBloomTime)
		// A disabled slice takes no hover, so the hub names nothing: it is
		// not a name that failed to appear, it is a slice that does nothing.
		if !act.enabled {
			if act.short != act.label {
				if got := floatingTextCount(tt.Texts(), act.short); got != 0 {
					t.Fatalf("a disabled slice put %q up %d times: %q", act.short, got, tt.Texts())
				}
			}
			continue
		}
		// Count the hub name, the accessible label when it matches, and
		// the persistent caption. Any extra copy would be a hover pill.
		want := 1
		if act.short == act.label {
			want = 2
		}
		if floatingCaption(act) == act.short {
			want++ // the persistent caption also names this action
		}
		if got := floatingTextCount(tt.Texts(), act.short); got != want {
			t.Fatalf("hovering %q puts %q up %d times, want %d: %q", act.label, act.short, got, want, tt.Texts())
		}
		// And nothing outside the hub is bigger than a slice's own button,
		// which is what a pill would be.
		for _, s := range tt.Texts() {
			r, ok := tt.Find(s)
			if !ok || inHub(r) {
				continue
			}
			if r.W > floatingSliceTarget+1 || r.H > floatingSliceTarget+1 {
				t.Fatalf("hovering %q draws %q at %+v, wider than a slice's own button", act.label, s, r)
			}
		}
	}
}
