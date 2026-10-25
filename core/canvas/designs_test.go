package canvas

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Many designs in one canvas: named by number, described, grouped by page
// and variant, copied as a variant, reordered, one picked, one removed.
func TestDesignsOfACanvas(t *testing.T) {
	s := Open(t.TempDir())
	c, _ := s.Create(proto.CanvasCreateParams{Title: "Landing"})
	edit := func(e proto.CanvasEdit) proto.Canvas {
		t.Helper()
		e.ID = c.ID
		got, _, _, err := s.Edit(e)
		if err != nil {
			t.Fatalf("%s: %v", e.Op, err)
		}
		return got
	}
	for _, v := range []string{"B", "C"} {
		edit(proto.CanvasEdit{Op: proto.CanvasOpFrameAdd, FrameInfo: &proto.CanvasFrame{Name: "Hero " + v, Page: "Home", Variant: v}})
	}
	edit(proto.CanvasEdit{Op: proto.CanvasOpFrameMeta, Frame: "1", Name: ptr("Hero A"), Description: ptr("Bold type, one call to action"), Page: ptr("Home"), Variant: ptr("A")})
	got := edit(proto.CanvasEdit{Op: proto.CanvasOpFrameCopy, Frame: "design-3"})
	if len(got.Frames) != 4 || got.Frames[3].Name != "Hero C (2)" || got.Frames[3].Variant != "C" || got.Frames[3].ID == got.Frames[2].ID {
		t.Fatalf("copy: %+v", got.Frames)
	}
	got = edit(proto.CanvasEdit{Op: proto.CanvasOpFrameMove, Frame: "4", Before: "1"})
	if got.Frames[0].Name != "Hero C (2)" || got.Frames[1].Description != "Bold type, one call to action" {
		t.Fatalf("move: %q %q", got.Frames[0].Name, got.Frames[1].Name)
	}
	got = edit(proto.CanvasEdit{Op: proto.CanvasOpFrameMeta, Frame: "desain-3", Favorite: ptr(true)})
	got = edit(proto.CanvasEdit{Op: proto.CanvasOpPick, Frame: "#3"})
	if !got.Frames[2].Favorite || got.Winner != got.Frames[2].ID {
		t.Fatalf("favorite, winner: %+v %q", got.Frames[2], got.Winner)
	}
	got = edit(proto.CanvasEdit{Op: proto.CanvasOpFrameRemove, Frame: "3"})
	if got.Winner != "" || len(got.Frames) != 3 {
		t.Fatalf("removed winner: %q", got.Winner)
	}
	if _, err := FrameIndex(got, "design-9"); err == nil || !strings.Contains(err.Error(), "no design 9") {
		t.Fatalf("design 9: %v", err)
	}
	link := Link(c.ID, 2)
	id, design, ok := ParseLink(link)
	if !ok || id != c.ID || design != "design-2" {
		t.Fatalf("link %q: %q %q", link, id, design)
	}
	if i, err := FrameIndex(got, design); err != nil || i != 1 {
		t.Fatalf("link's design: %d %v", i, err)
	}
}
