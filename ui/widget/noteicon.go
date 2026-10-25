package widget

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
)

// NoteIcon draws a note's or a group's icon value (core/iconlib) size
// high: its SVG, in its own color or ink; an emoji of before icons were
// SVG as text; and none as fallback in ink (the note icon when nil).
func NoteIcon(c *ui.Context, ref string, size float32, ink ui.Color, fallback *ui.SVG) ui.Element {
	svg, color, colored := icon.Note(ref)
	switch {
	case svg != nil:
		if colored {
			ink = color
		}
		return ui.Image(c, svg).Size(size, size).TextColor(ink).Shrink(0)
	case ref != "":
		return ui.Text(c, ref).FontSize(size * 0.92).Shrink(0)
	}
	if fallback == nil {
		fallback = icon.NoteDefault
	}
	return ui.Icon(c, fallback).FontSize(size).TextColor(ink).Shrink(0)
}
