package diffview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// loaded is a view showing a worktree with a commit and changes to commit.
func loaded() *View {
	v := New()
	v.loaded = true
	v.status = proto.ReviewStatus{Branch: "feat/x", Base: "main", Bases: []string{"main"}, Worktree: true,
		Ahead: 1, Uncommitted: 2, Remote: "origin", HasGH: true}
	v.setDiff(proto.Diff{Files: []proto.DiffFile{
		{Path: "a.go", Status: proto.FileModified, Added: 1, Removed: 1, Hunks: []proto.DiffHunk{{
			Header: "@@ -1,2 +1,2 @@",
			Lines: []proto.DiffLine{
				{Kind: ' ', Text: "package a", Old: 1, New: 1},
				{Kind: '-', Text: "var x = 1", Old: 2},
				{Kind: '+', Text: "var x = 2", New: 2},
			},
		}}},
		{Path: "logo.png", Status: proto.FileAdded, Binary: true},
	}})
	return v
}

func TestViewListsFilesAndOpensOne(t *testing.T) {
	v := loaded()
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, Options{Notify: func(string) {}}) }, 700, 500)
	tt.Frame()
	for _, want := range []string{"feat/x", "vs main", "2 uncommitted", "CHANGES", "a.go", "logo.png", "Commit…", "Push", "Pull request…"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	// Merging waits for the changes to be committed.
	if tt.HasText("Merge into main") {
		t.Error("offers to merge uncommitted changes")
	}
	if tt.HasText("var x = 2") {
		t.Error("the list shows a file's lines")
	}

	// A file opens on its lines, and goes back to the list.
	if err := tt.Click("a.go"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("var x = 2") || !tt.HasText("var x = 1") {
		t.Fatalf("the open file shows %q", tt.Texts())
	}
	if err := tt.Click("All changes"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("var x = 2") || !tt.HasText("logo.png") {
		t.Fatalf("back at the list: %q", tt.Texts())
	}

	if err := tt.Click("Commit…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Commit changes") {
		t.Errorf("no commit dialog: %q", tt.Texts())
	}
}

func TestAskSaysWhyAnActionDoesntFit(t *testing.T) {
	v := loaded()
	v.status.HasGH = false
	v.Ask(ActPR)
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, Options{Notify: func(string) {}}) }, 1100, 500)
	tt.Frame()
	if !tt.HasText("Opening a pull request needs the GitHub CLI (gh) on this host.") {
		t.Errorf("no reason shown: %q", tt.Texts())
	}
}
