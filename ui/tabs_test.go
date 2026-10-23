package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// A session's tabs show in its strip, not in the session list, and
// picking one from elsewhere brings it to the front of its session.
func TestTabsShowInTheirSession(t *testing.T) {
	a := New(Options{})
	parent := proto.SessionInfo{ID: "s1", Project: "/code/app", Agent: "claude"}
	tab := proto.SessionInfo{ID: "s2", Project: "/code/app", Agent: "shell", Parent: "s1"}
	a.sessions = []proto.SessionInfo{parent, tab}
	a.agents = []proto.AgentInfo{{ID: "claude", Name: "Claude Code"}, {ID: "shell", Name: "Shell"}}

	if got := a.projectSessions("/code/app"); len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("project sessions %+v", got)
	}
	tt := ui.NewTester(func(c *ui.Context) { a.tabStrip(c, parent) }, 600, 40)
	tt.Frame()
	if !tt.HasText("Claude Code") || !tt.HasText("Shell") {
		t.Fatalf("strip %q", tt.Texts())
	}
	if a.frontTab(parent).ID != "s1" {
		t.Fatal("a tab in front before one was picked")
	}
	a.selectSession(tab)
	if a.selected != "s1" || a.frontTab(parent).ID != "s2" {
		t.Fatalf("selected %q, front %q", a.selected, a.frontTab(parent).ID)
	}
	a.remove("s2")
	if a.frontTab(parent).ID != "s1" {
		t.Fatal("a removed tab stays in front")
	}
}
