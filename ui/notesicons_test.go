package ui

import (
	"github.com/enowx/enowx/ui/theme"
	"image/color"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A page's icon is picked from the SVG libraries, found by a word, then
// is among the recent ones; or is the user's own SVG, cleaned. Icons in
// the text and drawings show as such.
func TestNotesIconsArePickedAsSVG(t *testing.T) {
	h := newNotesHarness(t)
	n, err := h.cl.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Launch"},
		Body: "Ship it :icon[lucide:rocket] today\n```svg\n<svg viewBox=\"0 0 40 20\"><script>x()</script><rect width=\"40\" height=\"20\"/></svg>\n```\n"})
	if err != nil {
		t.Fatal(err)
	}
	h.settle("the page listed", func() bool { return h.tt.HasText("Launch") })
	if err := h.tt.Click("Launch"); err != nil {
		t.Fatal(err)
	}
	h.settle("the page open", func() bool { return h.tt.HasText("today") })
	if h.tt.HasText(":icon[lucide:rocket]") {
		t.Fatal("the icon shows as its code")
	}

	if err := h.tt.Click("Page icon"); err != nil {
		t.Fatal(err)
	}
	h.settle("the picker", func() bool { return h.tt.HasText("Library") })
	if err := h.tt.Click("Search icons: rocket, bug, calendar…"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("rocket")
	h.settle("rocket found", func() bool { _, ok := h.tt.Find("Icon lucide:rocket"); return ok })
	shot(t, h.tt, "notes-icon-picker.png")
	if err := h.tt.Click("Icon lucide:rocket"); err != nil {
		t.Fatal(err)
	}
	h.settle("the icon saved", func() bool {
		got, err := h.cl.Note(n.ID)
		return err == nil && got.Icon == "lucide:rocket"
	})

	// Picked, it is among the recent icons; the user's own SVG is cleaned.
	h.tt.Click("Page icon") // closes it
	h.tt.Frame()
	h.tt.Click("Page icon")
	h.settle("the picker again", func() bool { return h.tt.HasText("Recent") })
	if err := h.tt.Click("Custom"); err != nil {
		t.Fatal(err)
	}
	if err := h.tt.Click("SVG code"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type(`<svg viewBox="0 0 10 10"><circle cx="5" cy="5" r="5" fill="#f00" onload="x()"/></svg>`)
	h.tt.Frame()
	if err := h.tt.Click("Use this icon"); err != nil {
		t.Fatal(err)
	}
	h.settle("the own icon saved", func() bool {
		got, err := h.cl.Note(n.ID)
		return err == nil && strings.HasPrefix(got.Icon, "svg:")
	})
	got, _ := h.cl.Note(n.ID)
	h.settle("the picker closed", func() bool { return !h.tt.HasText("Library") })
	shot(t, h.tt, "notes-icons.png")
	if strings.Contains(got.Icon, "onload") {
		t.Errorf("kept a handler: %s", got.Icon)
	}
}

// A page's icon takes the size picked, and, with no color of its own, the
// theme's text color, changing with the theme.
func TestNoteIconSizeAndThemeColor(t *testing.T) {
	h := newNotesHarness(t)
	n, err := h.cl.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Launch", Icon: "lucide:rocket"}, Body: "text\n"})
	if err != nil {
		t.Fatal(err)
	}
	h.settle("the page listed", func() bool { return h.tt.HasText("Launch") })
	h.tt.Click("Launch")
	h.settle("the page open", func() bool { return h.tt.HasText("text") })
	small, _ := h.tt.Find("Page icon")
	inkOf := func() color.RGBA {
		r, _ := h.tt.Find("Page icon")
		img := h.tt.Image()
		bg := img.RGBAAt(int(r.X)+2, int(r.Y)+2)
		counts := map[color.RGBA]int{}
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if px := img.RGBAAt(x, y); px != bg {
					counts[px]++
				}
			}
		}
		var best color.RGBA
		for px, n := range counts {
			if n > counts[best] {
				best = px
			}
		}
		return best
	}
	before := inkOf()

	if err := h.tt.Click("Page icon"); err != nil {
		t.Fatal(err)
	}
	h.settle("the picker", func() bool { _, ok := h.tt.Find("Large"); return ok })
	if err := h.tt.Click("Large"); err != nil {
		t.Fatal(err)
	}
	h.settle("the size kept", func() bool { got, _ := h.cl.Note(n.ID); return got.Icon == "lucide:rocket@56" })
	h.tt.Click("Page icon") // closes the picker
	h.settle("the larger icon", func() bool { r, _ := h.tt.Find("Page icon"); return r.W > small.W+15 })

	old := theme.Current()
	t.Cleanup(func() { theme.Apply(old) })
	theme.Apply(theme.Midnight)
	h.tt.Frame()
	if after := inkOf(); after == before {
		t.Errorf("the icon kept its color %v across themes", before)
	}
}
