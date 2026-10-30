package ui

import (
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// A submenu must accept a quick second click, even after its parent has
// stopped animating and no hover frames are pending.
func TestFloatingFanRespondsQuickly(t *testing.T) {
	for _, fan := range []string{"Active sessions", "Panels"} {
		t.Run(fan, func(t *testing.T) {
			app, tt := floatingWithSessions(t)
			x, y := floatingCenter(t, tt, fan)
			tt.Move(x, y)
			tt.Advance(time.Second)
			if err := tt.Click(fan); err != nil {
				t.Fatal(err)
			}
			tt.Advance(120 * time.Millisecond)
			child := "This PC"
			if fan == "Panels" {
				child = "Show or hide the sidebar"
			}
			if err := tt.Click(child); err != nil {
				t.Fatal(err)
			}
			if fan == "Panels" {
				if !app.state.HideSessions {
					t.Fatal("the submenu ignored a click after 120 ms")
				}
			} else if !tt.HasText("Waiting one") {
				t.Fatal("the source submenu ignored a click after 120 ms")
			}
		})
	}
}

// The first wedge opens the two sources, and closes them again: it runs
// nothing itself.
func TestFloatingSourcesFan(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if tt.HasText("VPS") {
		t.Fatalf("the fan is open before its wedge was clicked: %q", tt.Texts())
	}
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	for _, want := range []string{"This PC", "VPS"} {
		if !tt.HasText(want) {
			t.Fatalf("the fan didn't open: %q", tt.Texts())
		}
	}
	if !app.floatingShown {
		t.Fatal("opening the fan folded the panel")
	}
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if tt.HasText("VPS") {
		t.Fatalf("the fan stayed open: %q", tt.Texts())
	}
	if !app.floatingShown {
		t.Fatal("closing the fan folded the panel")
	}
}

// One fan at a time: opening the panel toggles closes the sources, and the
// other way round.
func TestFloatingOneFanAtATime(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if !tt.HasText("VPS") {
		t.Fatalf("the sources didn't open: %q", tt.Texts())
	}
	if err := tt.Click("Panels"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if tt.HasText("VPS") {
		t.Fatalf("the sources stayed open under the panels: %q", tt.Texts())
	}
	if !tt.HasText("Show or hide the sidebar") {
		t.Fatalf("the panels didn't open: %q", tt.Texts())
	}
	_ = app
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if tt.HasText("Show or hide the sidebar") {
		t.Fatalf("the panels stayed open under the sources: %q", tt.Texts())
	}
	if !tt.HasText("VPS") {
		t.Fatalf("the sources didn't open: %q", tt.Texts())
	}
}

// Picking This PC turns the pie into that machine's sessions, and no action
// is left on it.
func TestFloatingThisPCSessions(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	for _, want := range []string{"Waiting one", "Working one", "Idle one"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q in This PC's sessions: %q", want, tt.Texts())
		}
	}
	for _, act := range app.floatingActions() {
		if tt.HasText(act.label) {
			t.Fatalf("the action %q stays on the pie in sessions: %q", act.label, tt.Texts())
		}
	}
	if tt.HasText("Server job") {
		t.Fatalf("the VPS's session shows under This PC: %q", tt.Texts())
	}
}

// The VPS from a fresh summon shows the servers' sessions, not this PC's.
func TestFloatingVPSSessions(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("VPS"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !tt.HasText("Server job") {
		t.Fatalf("no VPS session: %q", tt.Texts())
	}
	if tt.HasText("Waiting one") {
		t.Fatalf("This PC's session shows under the VPS: %q", tt.Texts())
	}
	_ = app
}

// Picking a source is always possible, and switching between them works: VPS
// while This PC has sessions, then back to This PC, which must show This PC's
// sessions again. A source with nothing to show opens on an empty pie that
// says so, rather than doing nothing.
func TestFloatingSourceSwitches(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("VPS"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !tt.HasText("Server job") {
		t.Fatalf("the VPS's session didn't show: %q", tt.Texts())
	}
	// Back to the actions, then This PC.
	if err := tt.Click("Back"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	for _, want := range []string{"Waiting one", "Working one", "Idle one"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q after switching back to This PC: %q", want, tt.Texts())
		}
	}
	if tt.HasText("Server job") {
		t.Fatalf("the VPS's session stays on This PC's pie: %q", tt.Texts())
	}
	_ = app
}

// A source whose sessions have all exited still opens: the ring is empty, the
// hub says so, and it goes back to the actions. A source is never a dead
// control.
func TestFloatingEmptySource(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	// One session, and it has ended.
	app.sessions = []proto.SessionInfo{
		{ID: "gone", Name: "Gone", Exited: true, Created: time.Now()},
	}
	tt := floatingTester(t, app)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	// Both sources are pickable, whether or not they have sessions.
	for _, src := range app.floatingSources() {
		if !src.enabled {
			t.Fatalf("%q is disabled", src.label)
		}
	}
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !tt.HasText(floatingEmptyEnded) {
		t.Fatalf("the empty source says nothing: %q", tt.Texts())
	}
	// The message stays inside the hub circle rather than running out over
	// the ring.
	r, ok := tt.Find(floatingEmptyEnded)
	if !ok {
		t.Fatalf("no %q text: %q", floatingEmptyEnded, tt.Texts())
	}
	cx, cy := float32(floatingSize)/2, float32(floatingSize)/2
	if r.X < cx-floatingHubSize/2 || r.Y < cy-floatingHubSize/2 ||
		r.X+r.W > cx+floatingHubSize/2 || r.Y+r.H > cy+floatingHubSize/2 {
		t.Fatalf("%q spills out of the hub: %+v", floatingEmptyEnded, r)
	}
	if tt.HasText("Gone") {
		t.Fatalf("an exited session is on the ring: %q", tt.Texts())
	}
	for _, act := range app.floatingActions() {
		if tt.HasText(act.label) {
			t.Fatalf("the action %q shows on an empty source: %q", act.label, tt.Texts())
		}
	}
	if err := tt.Click("Back"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !tt.HasText("New session") {
		t.Fatalf("Back doesn't return to the actions from an empty source: %q", tt.Texts())
	}
}

// A source with no sessions at all says so in its own words, not the ones for
// a source whose sessions all ended.
func TestFloatingSourceWithNothing(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	tt := floatingTester(t, app)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !tt.HasText(floatingEmptyNone) {
		t.Fatalf("a source with nothing doesn't say so: %q", tt.Texts())
	}
	if tt.HasText(floatingEmptyEnded) {
		t.Fatalf("a source with nothing reads as one that all ended: %q", tt.Texts())
	}
}
