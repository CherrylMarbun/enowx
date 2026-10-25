package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// The tray's panel lists the sessions needing the user first, then those
// working, then those done; not tabs, idle or ended ones.
func TestTrayPanelListsWhatNeedsTheUser(t *testing.T) {
	app := New(Options{})
	now := time.Now()
	app.sessions = []proto.SessionInfo{
		{ID: "done1", Name: "Docs", Project: "/p/site", State: proto.StateDone, Created: now.Add(-3 * time.Minute)},
		{ID: "work1", Name: "Parser", Project: "/p/api", State: proto.StateWorking, Active: true, Created: now.Add(-2 * time.Minute)},
		{ID: "wait1", Name: "Login fix", Project: "/p/api", State: proto.StateWaiting, Created: now.Add(-time.Minute)},
		{ID: "tab1", Parent: "work1", State: proto.StateWaiting},
		{ID: "gone", Name: "Old", State: proto.StateWaiting, Exited: true},
		{ID: "idle", Name: "Idle"},
	}
	var ids []string
	for _, s := range app.traySessionList() {
		ids = append(ids, s.ID)
	}
	if want := []string{"wait1", "work1", "done1"}; !equalStrings(ids, want) {
		t.Fatalf("listed %v, want %v", ids, want)
	}
	tt := ui.NewTester(app.trayView, 340, app.trayPanelHeight())
	tt.Frame()
	for _, want := range []string{"Login fix", "api · needs input", "Parser", "api · working", "Docs", "site · done", "Open enowX", "Quit"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	shot(t, tt, "tray-panel.png")
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
