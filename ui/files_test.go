package ui

import (
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

func TestFileTabsBelongToTheirSession(t *testing.T) {
	a := &App{client: &client.Client{}}
	s1, s2 := proto.SessionInfo{ID: "s1"}, proto.SessionInfo{ID: "s2"}
	a.selected = "s1"
	a.openFileTab(s1, "src/main.go")
	a.openFileTab(s1, "README.md")
	a.openFileTab(s1, "src/main.go") // already open: shown, not opened twice
	if got := a.fileTabs["s1"]; len(got) != 2 {
		t.Fatalf("tabs = %q", got)
	}
	if !a.panelShows("file:src/main.go") {
		t.Fatalf("shows %q", a.panelTab())
	}
	// Another session hasn't that file open: the panel shows Files.
	a.selected = "s2"
	if got := a.panelTab(); got != panelFiles {
		t.Fatalf("for s2 the panel shows %q", got)
	}
	a.selected = "s1"
	a.closeFileTab("s1", "src/main.go")
	if got := a.panelTab(); got != panelFiles || len(a.fileTabs["s1"]) != 1 {
		t.Fatalf("after closing the tab in front: %q, tabs %q", got, a.fileTabs["s1"])
	}
	_ = s2
}
