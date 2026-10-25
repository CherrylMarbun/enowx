package notesview

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func newTester(e *editor) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(600).Height(500).Children(func() { e.draw(c) })
	}, 600, 500)
}

// Keys a notebook's editor takes: Tab indents a list item, Backspace at a
// block's start turns it to text then joins it to the block before, a
// click ticks a to-do, and lines pasted make blocks.
func TestEditorKeys(t *testing.T) {
	e := newEditor("- one\n- two\njoin me\n- [ ] task\n")
	tt := newTester(e)
	tt.Frame()

	e.openAt(1, 0)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if got := e.markdown(); got != "- one\n  - two\njoin me\n- [ ] task\n" {
		t.Fatalf("after Tab: %q", got)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Frame()

	e.openAt(2, 0)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if got := e.markdown(); got != "- one\n- twojoin me\n- [ ] task\n" {
		t.Fatalf("after Backspace: %q", got)
	}
	if e.open != 1 || e.seen != 3 {
		t.Fatalf("open %d at %d, want 1 at 3", e.open, e.seen)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	if err := tt.Click("Done"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := e.markdown(); got != "- one\n- twojoin me\n- [x] task\n" {
		t.Fatalf("after ticking: %q", got)
	}

	e.insert(len(e.blocks)-1, block{})
	tt.Frame()
	tt.SetClipboard("## Pasted\nline one\nline two")
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if got := e.markdown(); got != "- one\n- twojoin me\n- [x] task\n## Pasted\nline one\nline two\n" {
		t.Fatalf("after pasting: %q", got)
	}
}

// A block dragged onto another goes before it.
func TestEditorMovesBlocksByDrag(t *testing.T) {
	e := newEditor("first\nsecond\nthird\n")
	tt := newTester(e)
	tt.Frame()
	from, _ := tt.Find("Block 3")
	to, _ := tt.Find("Block 1")
	tt.Press(from.X+20, from.Y+from.H/2)
	for k := 1; k <= 6; k++ {
		tt.Move(from.X+20, from.Y+from.H/2+(to.Y+4-from.Y-from.H/2)*float32(k)/6)
		tt.Frame()
	}
	tt.Release(to.X+20, to.Y+4)
	tt.Frame()
	if got := e.markdown(); got != "third\nfirst\nsecond\n" {
		t.Fatalf("after dragging: %q", got)
	}
}

// A click opens a block as text, code too.
func TestEditorOpensClickedBlocks(t *testing.T) {
	e := newEditor("some text\n```go\nx := 1\n```\n")
	tt := newTester(e)
	tt.Frame()
	if err := tt.Click("some text"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if e.open != 0 {
		t.Fatalf("open %d after clicking the text", e.open)
	}
	if err := tt.Click("x := 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if e.open != 1 {
		t.Fatalf("open %d after clicking the code", e.open)
	}
}
