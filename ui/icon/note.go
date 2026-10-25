package icon

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/iconlib"
)

// noteIcon is an icon value drawn: its SVG (nil when the value isn't an
// icon) and its own color.
type noteIcon struct {
	svg     *ui.SVG
	color   ui.Color
	colored bool
}

// noteIcons are the icon values drawn so far, by value, parsed once.
var noteIcons sync.Map

// Note is the drawing of icon value ref (core/iconlib), with its color
// when it has one of its own; nil for "" or a value that isn't an icon
// (an emoji of before icons were SVG).
func Note(ref string) (svg *ui.SVG, color ui.Color, colored bool) {
	if ref == "" {
		return nil, ui.Color{}, false
	}
	if v, ok := noteIcons.Load(ref); ok {
		n := v.(noteIcon)
		return n.svg, n.color, n.colored
	}
	var n noteIcon
	if r, err := iconlib.Parse(ref); err == nil {
		n.svg, _ = ui.ParseSVG([]byte(r.SVG()))
		if r.Color != "" {
			n.color, n.colored = ui.Hex(r.Color), true
		}
	}
	noteIcons.Store(ref, n)
	return n.svg, n.color, n.colored
}

// NoteDefault is the icon of a note without one of its own.
var NoteDefault, _, _ = Note(iconlib.Default)
