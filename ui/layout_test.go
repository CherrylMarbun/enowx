package ui

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestLayoutIsKeptPerProjectAcrossLaunches(t *testing.T) {
	file := filepath.Join(t.TempDir(), "app.json")
	a := &App{opts: Options{StateFile: file}}
	a.host.ID, a.project = "local", "/code/app"
	sessions := []proto.SessionInfo{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}

	a.setSplit(true)
	a.setLayout(func(l *layout) { l.Columns = 2 })
	a.hideFromSplit("s2", true)
	if got := a.splitSessions(sessions); len(got) != 2 || got[1].ID != "s3" {
		t.Fatalf("split shows %+v", got)
	}
	// Another project keeps its own.
	a.project = "/code/other"
	if a.splitOn() {
		t.Fatal("another project is split too")
	}
	a.project = "/code/app"

	a.saveState()
	b := &App{state: loadState(file)}
	b.host.ID, b.project = "local", "/code/app"
	if l := b.layout(); !l.Split || l.Columns != 2 || !slices.Equal(l.Hidden, []string{"s2"}) {
		t.Fatalf("after a relaunch: %+v", l)
	}

	// Leaving out every session shows them all; a deleted one is forgotten.
	b.hideFromSplit("s1", true)
	b.hideFromSplit("s3", true)
	if got := b.splitSessions(sessions); len(got) != 3 {
		t.Fatalf("with all left out, split shows %+v", got)
	}
	b.forgetInLayouts("s2")
	if l := b.layout(); slices.Contains(l.Hidden, "s2") {
		t.Fatalf("a removed session stays in the layout: %+v", l)
	}
}
