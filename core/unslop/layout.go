package unslop

import (
	"fmt"

	"github.com/enowx/enowx/proto"
)

// A page measured at a device's width, in a browser: what a static read of
// its styles can't tell, as its layout at that width decides it.

// Layout is a page as laid out Width CSS pixels wide: how far it scrolls
// sideways (Overflow, pixels), and the elements (selectors) that stick
// out past its width, take touches under 44 px, or set text under 12 px.
type Layout struct {
	Width    int      `json:"width"`
	Overflow int      `json:"overflow"`
	Off      []string `json:"off"`
	Small    []string `json:"small"`
	Tiny     []string `json:"tiny"`
}

// device names a width as a device's.
func device(w int) string {
	switch {
	case w <= 480:
		return fmt.Sprintf("phone (%d px)", w)
	case w <= 1024:
		return fmt.Sprintf("tablet (%d px)", w)
	}
	return fmt.Sprintf("%d px", w)
}

// LayoutFindings are what l shows wrong at its width: all the mobile
// pass's, the checks a phone or a tablet calls for.
func LayoutFindings(l Layout) []proto.UnslopFinding {
	var out []proto.UnslopFinding
	at := device(l.Width)
	if l.Overflow > 1 || len(l.Off) > 0 {
		ex := []string{fmt.Sprintf("scrolls %d px sideways", l.Overflow)}
		f := finding("mobile", "overflow", proto.UnslopHigh, "Wider than the screen on a "+at,
			"Let it fit its width: no fixed widths past it, rows that wrap, long words and images held in; a carousel scrolls in its own box.", append(ex, l.Off...), l.Off...)
		out = append(out, f)
	}
	if len(l.Small) > 0 {
		out = append(out, finding("mobile", "tap-targets", proto.UnslopMedium, "Touch targets under 44 px on a "+at,
			"Give each control 44 px to touch (padding counts), 8 px from the next.", l.Small, l.Small...))
	}
	if len(l.Tiny) > 0 {
		out = append(out, finding("mobile", "small-text", proto.UnslopMedium, "Text under 12 px on a "+at,
			"Keep text 12 px and up on small screens (inputs 16 px, so they don't zoom).", l.Tiny, l.Tiny...))
	}
	return out
}
