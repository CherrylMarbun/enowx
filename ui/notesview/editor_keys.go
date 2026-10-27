package notesview

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// The open block's keys, which come between frames (OnTextKey).

// key takes the keys that move between blocks, split, join and indent
// them, and reports whether it did.
func (e *editor) key(i int, k ui.TextKey) bool {
	if i != e.open || i >= len(e.blocks) {
		return false
	}
	b := &e.blocks[i]
	text := k.Text()
	if e.slashOpen() {
		if done, took := e.slashKey(i, k); took {
			return done
		}
	}
	code := b.kind == kCode || b.kind == kTable || b.kind == kLayout
	switch {
	case k.Key == ui.KeyEscape:
		e.close()
		return true
	case code:
		// Code takes Enter and Tab; Cmd+Enter leaves it for a new block.
		if k.Key == ui.KeyEnter && k.Mods != 0 {
			e.insert(i, block{})
			return true
		}
		return false
	case k.Key == ui.KeyEnter && k.Mods == 0:
		e.enter(i, k, text)
		return true
	case k.Key == ui.KeyBackspace && k.Mods == 0 && e.seen == 0 && !e.seenSel:
		if i == 0 && text == "" && b.kind == kText && e.leadingBackspace != nil {
			return e.leadingBackspace()
		}
		return e.backspace(i, text)
	case k.Key == ui.KeyDelete && k.Mods == 0 && e.seen == utf8.RuneCountInString(text) && !e.seenSel && i+1 < len(e.blocks):
		if _, ok := imageBlockParts(e.blocks[i+1]); ok {
			e.imageRemoval = &imageRemoval{block: i + 1, part: 1}
			return true
		}
	case k.Key == ui.KeyUp && k.Mods == 0 && e.seen == 0 && i > 0:
		e.openAt(prevTyped(e.blocks, i), -1)
		return true
	case k.Key == ui.KeyDown && k.Mods == 0 && e.seen == utf8.RuneCountInString(text) && i+1 < len(e.blocks):
		e.openAt(nextTyped(e.blocks, i), 0)
		return true
	case k.Key == ui.KeyTab && (k.Mods == 0 || k.Mods == ui.Shift):
		if b.isList() {
			if k.Mods == ui.Shift {
				b.indent = max(b.indent-1, 0)
			} else if i > 0 && e.blocks[i-1].isList() {
				b.indent = min(b.indent+1, e.blocks[i-1].indent+1)
			}
			e.changed = true
		}
		return true // Tab stays in the page
	}
	return false
}

// enter splits block i at the caret: what follows goes to a block after
// it. An empty list item ends its list; "```" and "---" make blocks.
func (e *editor) enter(i int, k ui.TextKey, text string) {
	b := &e.blocks[i]
	b.text = text
	switch {
	case b.kind == kText && strings.HasPrefix(text, "```"):
		b.kind, b.lang, b.text = kCode, strings.TrimSpace(text[3:]), ""
		e.changed = true
		k.SetText("")
		e.openAt(i, 0)
		return
	case b.kind == kText && (text == "---" || text == "***"):
		b.kind, b.text = kDivider, ""
		e.insert(i, block{})
		return
	case text == "" && b.kind != kText:
		if b.indent > 0 {
			b.indent--
		} else {
			b.kind = kText
		}
		e.changed = true
		e.openAt(i, 0)
		return
	}
	runes := []rune(text)
	at := min(e.seen, len(runes))
	if e.seen == 0 && len(runes) > 0 {
		// At its start: an empty block before it, as a notebook does.
		e.blocks = slices.Insert(e.blocks, i, block{kind: b.kind, indent: b.indent})
		if !b.isList() {
			e.blocks[i].kind = kText
		}
		e.changed = true
		e.openAt(i+1, 0)
		return
	}
	before, after := string(runes[:at]), string(runes[at:])
	k.SetText(before)
	b.text = before
	nb := b.next()
	nb.text = after
	e.insert(i, nb)
}

// backspace at a block's start: a list item outdents, then any kind
// turns to text, then text joins the block before it.
func (e *editor) backspace(i int, text string) bool {
	b := &e.blocks[i]
	switch {
	case b.isList() && b.indent > 0:
		b.indent--
	case b.kind != kText:
		b.kind, b.done = kText, false
	case i == 0:
		return false
	default:
		prev := &e.blocks[i-1]
		if parts, ok := imageBlockParts(*prev); ok {
			e.imageRemoval = &imageRemoval{block: i - 1, part: len(parts) - 2}
			return true
		}
		switch prev.kind {
		case kDivider:
			e.blocks = slices.Delete(e.blocks, i-1, i)
			e.changed = true
			e.openAt(i-1, 0)
			return true
		case kCode, kTable:
			if text == "" {
				e.remove(i)
			}
			e.openAt(i-1, -1)
			return true
		}
		join := utf8.RuneCountInString(prev.text)
		prev.text += text
		e.remove(i)
		e.openAt(i-1, join)
		return true
	}
	e.changed = true
	e.openAt(i, 0)
	return true
}

// prevTyped and nextTyped are the blocks Up and Down go to, over dividers.
func prevTyped(bs []block, i int) int {
	for j := i - 1; j >= 0; j-- {
		if bs[j].kind != kDivider {
			return j
		}
	}
	return i
}

func nextTyped(bs []block, i int) int {
	for j := i + 1; j < len(bs); j++ {
		if bs[j].kind != kDivider {
			return j
		}
	}
	return i
}
