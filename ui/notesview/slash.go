package notesview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// The slash menu: "/" typed in an empty text block lists the kinds of
// block, narrowed by what follows it; Enter or a click makes the block
// that kind.

type slashItem struct {
	name, hint string
	make       block
}

var slashItems = []slashItem{
	{"Text", "Plain text", block{kind: kText}},
	{"Heading 1", "#", block{kind: kH1}},
	{"Heading 2", "##", block{kind: kH2}},
	{"Heading 3", "###", block{kind: kH3}},
	{"To-do list", "[]", block{kind: kTodo}},
	{"Bulleted list", "-", block{kind: kBullet}},
	{"Numbered list", "1.", block{kind: kNumber}},
	{"Quote", ">", block{kind: kQuote}},
	{"Code", "```", block{kind: kCode}},
	{"Table", "|", block{kind: kTable, text: "| Column | Column |\n|---|---|\n|  |  |"}},
	{"Divider", "---", block{kind: kDivider}},
}

// slashOpen says the open block shows the menu: text starting with "/",
// one word.
func (e *editor) slashOpen() bool {
	if e.open < 0 || e.open >= len(e.blocks) {
		return false
	}
	b := e.blocks[e.open]
	return b.kind == kText && strings.HasPrefix(b.text, "/") && !strings.ContainsAny(b.text, " \n")
}

// slashMatches are the items the open block's text after "/" names.
func (e *editor) slashMatches() []slashItem {
	q := strings.ToLower(strings.TrimPrefix(e.blocks[e.open].text, "/"))
	var out []slashItem
	for _, it := range slashItems {
		if strings.Contains(strings.ToLower(it.name), q) {
			out = append(out, it)
		}
	}
	return out
}

// slashMenu shows the menu under the open block's input.
func (e *editor) slashMenu(c *ui.Context, in ui.Element, i int) {
	items := e.slashMatches()
	if len(items) == 0 {
		e.menuIn = false
		return
	}
	e.slash = min(e.slash, len(items)-1)
	ui.Overlay(c, func() {
		menu := ui.Column(c).AttachTo(in, ui.AnchorBottomStart, ui.AnchorTopStart).Width(240).Padding(4).Gap(1).
			Margin(4, 0, 0, 0).Radius(8).Background(theme.Raised).Border(1, theme.Border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.35))
		e.menuIn = menu.Hovered()
		menu.Children(func() {
			ui.Text(c, "Blocks").FontSize(10.5).TextColor(theme.Muted).Padding(4, 8)
			for j, it := range items {
				row := ui.ButtonBase(c.Key(it.name)).FillWidth().Height(30).PaddingX(8).Gap(8).Radius(5).Cursor(ui.CursorPointer).Label(it.name)
				if j == e.slash || row.Hovered() {
					row.Background(theme.Hover)
				}
				row.Children(func() {
					ui.Text(c, it.name).FontSize(12.5).TextColor(theme.Text).Grow(1)
					ui.Text(c, it.hint).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted)
				})
				if row.Clicked() {
					e.makeBlock(i, it)
				}
			}
		})
	})
}

// slashKey takes the menu's keys: Up and Down pick, Enter makes, Escape
// leaves the text as typed. took says the menu had the key.
func (e *editor) slashKey(i int, k ui.TextKey) (done, took bool) {
	items := e.slashMatches()
	if len(items) == 0 {
		return false, false
	}
	switch k.Key {
	case ui.KeyUp:
		e.slash = (e.slash + len(items) - 1) % len(items)
	case ui.KeyDown:
		e.slash = (e.slash + 1) % len(items)
	case ui.KeyEnter:
		k.SetText("")
		e.makeBlock(i, items[min(e.slash, len(items)-1)])
	case ui.KeyEscape:
		k.SetText(strings.TrimPrefix(k.Text(), "/"))
		e.blocks[i].text = strings.TrimPrefix(e.blocks[i].text, "/")
	default:
		return false, false
	}
	return true, true
}

// makeBlock makes block i the item's kind, empty.
func (e *editor) makeBlock(i int, it slashItem) {
	e.blocks[i] = it.make
	e.changed, e.menuIn = true, false
	if it.make.kind == kDivider {
		e.insert(i, block{})
		return
	}
	e.openAt(i, -1)
}
