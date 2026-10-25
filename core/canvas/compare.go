package canvas

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/enowx/enowx/proto"
)

// Change is a difference between two versions of a canvas: what was
// Added, Removed or Changed, in frame Frame (its name), element Element
// ("" for the frame itself).
type Change struct {
	Kind    string // "added", "removed", "changed"
	Frame   string
	Element string
	About   string
}

func (c Change) String() string {
	what := "frame " + c.Frame
	if c.Element != "" {
		what = c.Element + " in " + c.Frame
	}
	if c.About != "" {
		what += " (" + c.About + ")"
	}
	return c.Kind + ": " + what
}

// Compare is what changed from canvas a to canvas b: frames, their
// shapes and their mockups' elements, by ID.
func Compare(a, b proto.Canvas) []Change {
	var out []Change
	for _, fb := range b.Frames {
		i := slices.IndexFunc(a.Frames, func(f proto.CanvasFrame) bool { return f.ID == fb.ID })
		if i < 0 {
			out = append(out, Change{Kind: "added", Frame: fb.Name})
			continue
		}
		out = append(out, compareFrame(a.Frames[i], fb)...)
	}
	for _, fa := range a.Frames {
		if !slices.ContainsFunc(b.Frames, func(f proto.CanvasFrame) bool { return f.ID == fa.ID }) {
			out = append(out, Change{Kind: "removed", Frame: fa.Name})
		}
	}
	return out
}

func compareFrame(a, b proto.CanvasFrame) []Change {
	var out []Change
	if a.Name != b.Name || a.Width != b.Width || a.Height != b.Height || a.Background != b.Background {
		out = append(out, Change{Kind: "changed", Frame: b.Name, About: fmt.Sprintf("%d×%d", b.Width, b.Height)})
	}
	for _, eb := range b.Elements {
		i := slices.IndexFunc(a.Elements, func(e proto.CanvasElement) bool { return e.ID == eb.ID })
		switch {
		case i < 0:
			out = append(out, Change{Kind: "added", Frame: b.Name, Element: eb.ID, About: eb.Type})
		case !reflect.DeepEqual(a.Elements[i], eb):
			out = append(out, Change{Kind: "changed", Frame: b.Name, Element: eb.ID, About: eb.Type})
		}
	}
	for _, ea := range a.Elements {
		if !slices.ContainsFunc(b.Elements, func(e proto.CanvasElement) bool { return e.ID == ea.ID }) {
			out = append(out, Change{Kind: "removed", Frame: b.Name, Element: ea.ID, About: ea.Type})
		}
	}
	if a.HTML == b.HTML {
		return out
	}
	oa, ob := Outline(a.HTML), Outline(b.HTML)
	seen := false
	for _, hb := range ob {
		i := slices.IndexFunc(oa, func(h HTMLElement) bool { return h.ID == hb.ID })
		if i < 0 {
			out, seen = append(out, Change{Kind: "added", Frame: b.Name, Element: "#" + hb.ID, About: hb.Tag}), true
			continue
		}
		x, _ := ElementHTML(a.HTML, hb.ID)
		y, _ := ElementHTML(b.HTML, hb.ID)
		if x != y && !containsChanged(ob, hb, a.HTML, b.HTML) {
			out, seen = append(out, Change{Kind: "changed", Frame: b.Name, Element: "#" + hb.ID, About: hb.Tag}), true
		}
	}
	for _, ha := range oa {
		if !slices.ContainsFunc(ob, func(h HTMLElement) bool { return h.ID == ha.ID }) {
			out, seen = append(out, Change{Kind: "removed", Frame: b.Name, Element: "#" + ha.ID, About: ha.Tag}), true
		}
	}
	if !seen {
		out = append(out, Change{Kind: "changed", Frame: b.Name, About: "its HTML"})
	}
	return out
}

// containsChanged says a child of h with an id changed, which says it
// better than h does: only the innermost changed elements are told.
func containsChanged(list []HTMLElement, h HTMLElement, a, b string) bool {
	i := slices.IndexFunc(list, func(x HTMLElement) bool { return x.ID == h.ID })
	for _, child := range list[i+1:] {
		if child.Depth <= h.Depth {
			break
		}
		x, errA := ElementHTML(a, child.ID)
		y, _ := ElementHTML(b, child.ID)
		if errA != nil || x != y {
			return true
		}
	}
	return false
}
