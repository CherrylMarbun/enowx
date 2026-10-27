package notesview

import (
	"slices"
	"strings"
)

type imageRemoval struct{ block, part int }

// applyImageRemoval runs after drawing so removing a row cannot invalidate
// the other blocks being visited. Companion text remains ordinary note blocks.
func (e *editor) applyImageRemoval() {
	r := e.imageRemoval
	e.imageRemoval = nil
	if r == nil || e.env.read || r.block < 0 || r.block >= len(e.blocks) {
		return
	}
	parts, ok := imageBlockParts(e.blocks[r.block])
	if !ok || r.part <= 0 || r.part >= len(parts)-1 || r.part%2 == 0 {
		return
	}
	var body string
	if e.blocks[r.block].kind == kLayout {
		body = parts[2].raw
	} else {
		var text strings.Builder
		for j, part := range parts {
			if j != r.part {
				text.WriteString(part.raw)
			}
		}
		body = text.String()
	}
	if strings.TrimSpace(body) == "" {
		e.remove(r.block)
	} else {
		e.blocks = slices.Replace(e.blocks, r.block, r.block+1, parseBlocks(body)...)
		e.changed = true
	}
	e.imageEditors = nil
	e.mentionActive = false
	e.openAt(min(r.block, len(e.blocks)-1), 0)
}
