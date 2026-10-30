package ui

import (
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// The ring shows what the user needs first: a session waiting for them,
// then one at work, then the rest, newest first within each.
func TestFloatingSessionsOrder(t *testing.T) {
	now := time.Now()
	got := floatingActiveSessions([]proto.SessionInfo{
		{ID: "i1", Created: now.Add(-time.Minute)},
		{ID: "i2", Created: now.Add(-2 * time.Minute)},
		{ID: "w1", State: proto.StateWorking, Created: now},
		{ID: "x", Exited: true, Created: now.Add(time.Hour)},
		{ID: "q1", State: proto.StateWaiting, Created: now.Add(-time.Hour)},
	})
	var ids []string
	for _, s := range got {
		ids = append(ids, s.ID)
	}
	want := []string{"q1", "w1", "i1", "i2"}
	if len(ids) != len(want) {
		t.Fatalf("order %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order %v, want %v", ids, want)
		}
	}
}

// More sessions than slices: the pie holds six, and the centre says how many
// it left out.
func TestFloatingSessionsTruncate(t *testing.T) {
	app := New(Options{})
	app.host = LocalHost()
	for i := range 8 {
		app.sessions = append(app.sessions, proto.SessionInfo{ID: "s" + strconv.Itoa(i), Name: "Job " + strconv.Itoa(i)})
	}
	tt := floatingTester(t, app)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	shown := 0
	for i := range 8 {
		if tt.HasText("Job " + strconv.Itoa(i)) {
			shown++
		}
	}
	if shown != floatingMaxSessions {
		t.Fatalf("%d sessions on the pie, want %d: %q", shown, floatingMaxSessions, tt.Texts())
	}
	// The count is the centre's tooltip: the pointer rests on it to read it.
	hx, hy := floatingCenter(t, tt, "Back")
	tt.Move(hx, hy)
	tt.Advance(time.Second)
	if !tt.HasText("2 more") {
		t.Fatalf("the centre doesn't say what it left out: %q", tt.Texts())
	}
}

// A session's slice carries its status as a dot and a tint, and the status is
// the one statusOf gives: a waiting session reads "needs input". No bubble
// carries it any more.
func TestFloatingSessionStatusShown(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	// The status is in the wedge's data, which is what colours its slice and
	// its dot; the hub still names the session.
	want := statusOf(app.sessions[2]) // "wait"
	if want.label != "needs input" {
		t.Fatalf("the waiting session's status is %q", want.label)
	}
	acts := app.floatingSessionWedges(app.floatingSessions(floatingThisPC))
	found := false
	for _, act := range acts {
		if act.status != nil && act.status.label == "needs input" {
			found = true
		}
	}
	if !found {
		t.Fatal("no wedge carries the waiting session's status")
	}
	if tt.HasText("needs input") {
		t.Fatalf("the status is drawn as text, not only as a dot: %q", tt.Texts())
	}
	x, y := floatingCenter(t, tt, "Waiting one")
	tt.Move(x, y)
	tt.Advance(time.Second)
	if !tt.HasText("Waiting one") {
		t.Fatalf("the hub doesn't name the session: %q", tt.Texts())
	}
	if st := statusOf(app.sessions[1]); st.label != "working" {
		t.Fatalf("the working session's status is %q", st.label)
	}
}

// A session's slice goes to its session and folds the panel, as an action
// does.
func TestFloatingSessionClickGoes(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.Click("Working one"); err != nil {
		t.Fatal(err)
	}
	if app.floatingShown {
		t.Fatal("going to a session left the panel up")
	}
	if app.selected != "work" {
		t.Fatalf("selected %q, want the session clicked", app.selected)
	}
}

// The centre does two jobs: on the actions it hides the panel, on a source's
// sessions it goes back to them.
func TestFloatingHubBackAndHide(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if err := tt.Click("Back"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !app.floatingShown {
		t.Fatal("Back folded the panel")
	}
	if !tt.HasText("New session") {
		t.Fatalf("Back didn't return to the actions: %q", tt.Texts())
	}
	if err := tt.Click("Hide the floating control"); err != nil {
		t.Fatal(err)
	}
	if app.floatingShown {
		t.Fatal("the centre doesn't hide the panel from the actions")
	}
}

// Escape walks back one level at a time: a source's sessions to the actions,
// a fan into its wedge, and only then the panel away.
func TestFloatingEscapeLevels(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(time.Second)
	if !app.floatingShown {
		t.Fatal("Escape folded the panel from the sessions")
	}
	if !tt.HasText("New session") {
		t.Fatalf("Escape didn't return to the actions: %q", tt.Texts())
	}
	if err := tt.Click("Panels"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	// Off the wedge: a tooltip under the pointer takes Escape first, as a
	// pop-up does, and this is about the panel's own levels.
	tt.Move(2, 2)
	tt.Advance(time.Second)
	tt.Key(0, ui.KeyEscape)
	tt.Advance(time.Second)
	if tt.HasText("Show or hide the sidebar") {
		t.Fatalf("Escape left the fan open: %q", tt.Texts())
	}
	if !app.floatingShown {
		t.Fatal("Escape folded the panel with the fan open")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if app.floatingShown {
		t.Fatal("Escape left the panel up")
	}
}

// A summon starts over: the actions, no source, no fan.
func TestFloatingSummonResets(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	if err := tt.Click("This PC"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	if !tt.HasText("Waiting one") {
		t.Fatalf("the sessions didn't show: %q", tt.Texts())
	}
	app.hideFloating()
	tt.Frame()
	app.showFloating() // the next summon
	tt.Frame()
	tt.Advance(time.Second)
	if !tt.HasText("New session") {
		t.Fatalf("a summon didn't start on the actions: %q", tt.Texts())
	}
	if tt.HasText("Waiting one") {
		t.Fatalf("a summon kept the sessions: %q", tt.Texts())
	}
	if tt.HasText("VPS") {
		t.Fatalf("a summon kept a fan open: %q", tt.Texts())
	}
}
