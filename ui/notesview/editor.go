package notesview

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// editor is a page's body being edited: its blocks, each shown as it
// reads but the one open as text. Its fields belong to the main thread;
// the keys of the open block's input (OnTextKey) come there too, between
// frames.
type editor struct {
	blocks []block
	open   int // the block open as text, -1 for none
	// caret is where the caret goes in the block opening (runes; -1 its
	// end); focusing says to put it there and focus it this frame.
	caret    int
	focusing bool
	// seen is the open input's caret as last drawn, which keys go by: an
	// input gives no caret with its keys.
	seen    int
	seenSel bool // a selection, not a caret
	slash   int  // the item picked in the slash menu
	menuIn  bool // the pointer is over the slash menu
	changed bool // since the page last read it
	// iconAt is the block the icon picker is open for (-1: none), its
	// caret iconCaret; picker draws it, iconOpened readies it.
	iconAt     int
	iconOpen   bool
	iconCaret  int
	picker     iconPickerFunc
	iconOpened func()
}

func newEditor(md string) *editor { return &editor{blocks: parseBlocks(md), open: -1, iconAt: -1} }

func (e *editor) markdown() string { return renderBlocks(e.blocks) }

// openAt opens block i as text, its caret at caret (-1: its end).
func (e *editor) openAt(i, caret int) {
	if i < 0 || i >= len(e.blocks) {
		return
	}
	if e.blocks[i].kind == kDivider {
		return // nothing to type in
	}
	e.open, e.caret, e.focusing, e.slash = i, caret, true, 0
}

func (e *editor) close() { e.open = -1 }

// insert puts b after block i, opened.
func (e *editor) insert(i int, b block) {
	e.blocks = slices.Insert(e.blocks, i+1, b)
	e.changed = true
	e.openAt(i+1, 0)
}

// remove drops block i, keeping one block at least.
func (e *editor) remove(i int) {
	e.blocks = slices.Delete(e.blocks, i, i+1)
	if len(e.blocks) == 0 {
		e.blocks = []block{{}}
	}
	e.changed = true
}

// draw builds the blocks, then the space below them, which a click types
// in: in the last block when it is empty text, else in a new one.
func (e *editor) draw(c *ui.Context) {
	numbers := e.numbers()
	ui.Column(c).FillWidth().Gap(2).Children(func() {
		for i := range e.blocks {
			if i == e.open {
				e.edit(c.Key(i), i, numbers[i])
			} else {
				e.show(c.Key(i), i, numbers[i])
			}
		}
		tail := ui.ButtonBase(c).FillWidth().Height(160).Cursor(ui.CursorText).Label("Type below")
		// A block dropped below the others goes last.
		if d, ok := ui.Drop[blockDrag](tail); ok {
			e.move(d.at, len(e.blocks))
		}
		if tail.Clicked() {
			last := len(e.blocks) - 1
			if b := e.blocks[last]; b.kind == kText && b.text == "" {
				e.openAt(last, 0)
			} else {
				e.insert(last, block{})
			}
		}
	})
}

// numbers are the numbers numbered items show, as renderBlocks writes them.
func (e *editor) numbers() []int {
	out := make([]int, len(e.blocks))
	at := map[int]int{}
	for i, b := range e.blocks {
		switch {
		case b.kind == kNumber:
			at[b.indent]++
			for d := range at {
				if d > b.indent {
					delete(at, d)
				}
			}
			out[i] = at[b.indent]
		case b.kind != kBullet && b.kind != kTodo:
			clear(at)
		}
	}
	return out
}

// textStyle is a block's text's size and weight, shown or typed.
func textStyle(k kind) (float32, int) {
	switch k {
	case kH1:
		return 26, 700
	case kH2:
		return 20.5, 650
	case kH3:
		return 16.5, 620
	}
	return 14, 400
}

// blockRow lays out a block: its margin, the mark before it (a list's
// bullet or number, a to-do's box, a quote's bar), then its text.
func (e *editor) blockRow(c *ui.Context, row ui.Element, i, n int, text func()) {
	b := &e.blocks[i]
	top := float32(0)
	switch b.kind {
	case kH1:
		top = 14
	case kH2:
		top = 10
	case kH3:
		top = 6
	}
	row.FillWidth().Gap(8).Padding(top+2, 6, 2, 6+float32(b.indent)*24).Radius(5).AlignItems(ui.Start).Children(func() {
		switch b.kind {
		case kBullet:
			// A dot in the middle of the first line.
			ui.Row(c).Width(14).Height(20).Shrink(0).Center().Children(func() {
				ui.Box(c).Size(5, 5).Radius(3).Background(theme.TextSoft)
			})
		case kNumber:
			ui.Text(c, strconv.Itoa(max(n, 1))+".").FontSize(14).TextColor(theme.TextSoft).MinWidth(14).Shrink(0)
		case kTodo:
			box := ui.ButtonBase(c).Size(16, 16).Margin(2, 0, 0, 0).Shrink(0).Radius(4).Center().Border(1.5, theme.BorderStrong).
				Cursor(ui.CursorPointer).Label("Done")
			if b.done {
				box.Background(theme.Accent).Border(1.5, theme.Accent).Children(func() {
					ui.Icon(c, icon.Check).FontSize(11).TextColor(theme.OnAccent)
				})
			}
			if box.Clicked() {
				b.done = !b.done
				e.changed = true
			}
		case kQuote:
			ui.Box(c).Width(3).FillHeight().MinHeight(20).Radius(2).Background(theme.BorderStrong).Shrink(0)
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(text)
	})
}

// blockDrag is what dragging a block carries: where it is.
type blockDrag struct{ at int }

// move puts block from before block to.
func (e *editor) move(from, to int) {
	if from == to || from+1 == to {
		return
	}
	b := e.blocks[from]
	e.blocks = slices.Delete(e.blocks, from, from+1)
	if to > from {
		to--
	}
	e.blocks = slices.Insert(e.blocks, to, b)
	e.changed = true
}

// show builds block i as it reads; a click opens it as text, a drag
// moves it before the block it is dropped on.
func (e *editor) show(c *ui.Context, i, n int) {
	b := &e.blocks[i]
	row := ui.ButtonBase(c).Cursor(ui.CursorText).Label("Block " + strconv.Itoa(i+1))
	row.Drag(blockDrag{i})
	if d, ok := ui.Drop[blockDrag](row); ok {
		e.move(d.at, i)
	}
	if d, ok := ui.DragOver[blockDrag](row); ok && d.at != i {
		row.BorderWidth(2, 0, 0, 0).BorderColor(theme.Accent)
	} else if row.Hovered() {
		row.Background(theme.Hover)
	}
	switch b.kind {
	case kDivider:
		row.FillWidth().Padding(10, 6).Children(func() {
			ui.Box(c).FillWidth().Height(1).Background(theme.Border)
		})
		if row.Clicked() {
			e.insert(i, block{})
		}
		return
	case kCode:
		row.FillWidth().Padding(4, 0).Children(func() {
			if b.lang == "svg" {
				drawingView(c, b.text)
			} else {
				codeView(c, b.lang, b.text)
			}
		})
	case kTable:
		row.FillWidth().Padding(4, 0).Children(func() { tableView(c, b.text) })
	default:
		e.blockRow(c, row, i, n, func() {
			size, weight := textStyle(b.kind)
			switch {
			case b.text == "":
				ui.Box(c).FillWidth().Height(size * 1.45)
			case b.kind == kTodo && b.done:
				ui.Text(c, b.text).FontSize(size).TextColor(theme.Muted).Strikethrough().FillWidth()
			case widget.HasInlineIcons(b.text):
				ink := theme.Text
				if b.kind == kQuote {
					ink = theme.TextSoft
				} else if b.kind != kText {
					ink = theme.TextStrong
				}
				widget.InlineIcons(c, b.text, size, weight, ink, b.kind == kQuote)
			default:
				t := widget.InlineLabel(c, b.text).FontSize(size).FontWeight(weight).FillWidth()
				if b.kind == kQuote {
					t.TextColor(theme.TextSoft).Italic()
				} else if b.kind != kText {
					t.TextColor(theme.TextStrong)
				}
			}
		})
	}
	if row.Clicked() {
		e.openAt(i, -1)
	}
}

// edit builds block i open as text, and takes its keys.
func (e *editor) edit(c *ui.Context, i, n int) {
	b := &e.blocks[i]
	var in ui.Element
	switch b.kind {
	case kCode, kTable:
		ui.Column(c).FillWidth().Gap(4).Padding(4, 0).Children(func() {
			if b.kind == kCode {
				widget.Input(c, &b.lang, "language").Width(160).Height(26).FontSize(11.5).Font(theme.MonoFont)
			}
			in = ui.TextAreaBase(c, &b.text).FillWidth().Lines(2, 40).Font(theme.MonoFont).FontSize(12.5).Padding(10, 12).
				Radius(8).Background(theme.Sunken).Border(1, theme.AccentLine).Label("Code")
		})
	default:
		row := ui.Row(c).Background(theme.Fade(theme.Hover, 0.6))
		e.blockRow(c, row, i, n, func() {
			size, weight := textStyle(b.kind)
			in = ui.TextAreaBase(c, &b.text).FillWidth().Lines(1, 60).FontSize(size).FontWeight(weight).
				Placeholder(placeholder(*b)).Label("Block text")
			if b.kind == kTodo && b.done {
				in.TextColor(theme.Muted)
			}
		})
	}
	in.OnTextKey(func(k ui.TextKey) bool { return e.key(i, k) })
	if e.focusing {
		in.Focus()
		end := utf8.RuneCountInString(b.text)
		at := e.caret
		if at < 0 || at > end {
			at = end
		}
		in.SetTextSelection(at, at)
		e.seen, e.seenSel, e.focusing = at, false, false
	} else {
		start, stop := in.TextSelection()
		e.seen, e.seenSel = start, start != stop
		if !in.Focused() && !e.menuIn && e.iconAt != i {
			e.close() // the focus went elsewhere
		}
	}
	if in.Changed() {
		e.changed = true
		e.typed(i)
	}
	if e.open == i && e.slashOpen() {
		e.slashMenu(c, in, i)
	} else {
		e.menuIn = false
	}
	e.inlineIconPicker(c, in, i)
}

func placeholder(b block) string {
	switch b.kind {
	case kH1:
		return "Heading 1"
	case kH2:
		return "Heading 2"
	case kH3:
		return "Heading 3"
	case kTodo:
		return "To-do"
	case kBullet, kNumber:
		return "List"
	case kQuote:
		return "Quote"
	}
	return "Type, or / for blocks"
}

// typed follows what was typed into block i: a Markdown shortcut makes it
// another kind, lines pasted make blocks.
func (e *editor) typed(i int) {
	b := &e.blocks[i]
	if b.kind != kCode && b.kind != kTable && strings.Contains(b.text, "\n") {
		parsed := parseBlocks(b.text)
		if b.kind != kText && parsed[0].kind == kText {
			parsed[0].kind, parsed[0].indent = b.kind, b.indent
		}
		e.blocks = slices.Replace(e.blocks, i, i+1, parsed...)
		e.openAt(i+len(parsed)-1, -1)
		return
	}
	if n := b.typedShortcut(); n > 0 {
		e.openAt(i, max(e.seen-n, 0))
	}
}
