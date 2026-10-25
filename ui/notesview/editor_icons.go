package notesview

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/iconlib"
)

// Icons in a page's text: /icon in a block opens the icon picker by it,
// and the icon picked goes in the text at the caret, as :icon[lib:name].

// iconPicker draws the picker by anchor while open, pick getting the icon
// chosen; the page gives it to its editor (View.pageView).
type iconPickerFunc func(c *ui.Context, anchor ui.Element, open *bool, pick func(ref string))

// pickInlineIcon opens the picker for an icon in block i's text, where
// its caret is.
func (e *editor) pickInlineIcon(i int) {
	e.iconAt, e.iconOpen, e.iconCaret = i, true, e.seen
	if e.iconOpened != nil {
		e.iconOpened()
	}
}

// inlineIconPicker draws the picker by block i's input while it is open
// for it.
func (e *editor) inlineIconPicker(c *ui.Context, in ui.Element, i int) {
	if e.iconAt != i || e.picker == nil {
		return
	}
	e.picker(c, in, &e.iconOpen, func(ref string) {
		e.iconOpen = false
		if ref == "" {
			return
		}
		b := &e.blocks[i]
		runes := []rune(b.text)
		at := min(max(e.iconCaret, 0), len(runes))
		ins := []rune(iconlib.Inline(ref) + " ")
		b.text = string(runes[:at]) + string(ins) + string(runes[at:])
		e.changed = true
		e.openAt(i, at+len(ins))
	})
	if !e.iconOpen {
		e.iconAt = -1
	}
}
