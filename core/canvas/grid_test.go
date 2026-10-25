package canvas

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A design's grid: 12 columns of a 1440 frame, 24 apart within 48 of its
// edges; shapes snap to its lines, its areas give CSS and a skeleton.
func TestDesignGrid(t *testing.T) {
	s := Open(t.TempDir())
	c, _ := s.Create(proto.CanvasCreateParams{Title: "Landing", Kind: proto.CanvasWireframe})
	g := &proto.CanvasGrid{Cols: 12, Rows: 6, ColGap: 24, RowGap: 24, Margin: 48, Snap: true, Areas: []proto.GridArea{
		{Name: "Header", Col: 1, Row: 1, ColSpan: 12},
		{Name: "hero", Col: 1, Row: 2, ColSpan: 8, RowSpan: 3},
		{Name: "aside", Col: 9, Row: 2, ColSpan: 4, RowSpan: 3},
	}}
	if _, _, _, err := s.Edit(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpFrameGrid, Grid: g}); err != nil {
		t.Fatal(err)
	}
	box := proto.CanvasElement{ID: "box", Type: proto.ShapeRect, X: 60, Y: 170, W: 300, H: 100}
	got, _, _, err := s.Edit(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpElementSet, Element: &box})
	if err != nil {
		t.Fatal(err)
	}
	f := got.Frames[0]
	cols, rows := Tracks(*f.Grid, f)
	if cols[0].Start != 48 || cols[0].Size != 90 || cols[1].Start != 162 {
		t.Fatalf("columns %+v", cols[:2])
	}
	el := f.Elements[0]
	// Its left edge on column 1's, its right on column 3's end, its rows'.
	if el.X != 48 || el.W != 3*90+2*24 || el.Y != rows[1].Start {
		t.Fatalf("snapped to %g,%g %g×%g; rows %+v", el.X, el.Y, el.W, el.H, rows[:2])
	}
	if f.Grid.Areas[0].Name != "header" {
		t.Fatalf("area named %q", f.Grid.Areas[0].Name)
	}
	css := GridCSS(*f.Grid)
	for _, want := range []string{"grid-template-columns: repeat(12, minmax(0, 1fr));", "column-gap: 24px;", "padding: 48px;", "#hero { grid-area: 2 / 1 / 5 / 9; }", "#aside { grid-area: 2 / 9 / 5 / 13; }"} {
		if !strings.Contains(css, want) {
			t.Fatalf("css lacks %q:\n%s", want, css)
		}
	}
	if html := GridHTML(*f.Grid); len(Outline(html)) != 4 {
		t.Fatalf("skeleton %v", Outline(html))
	}
	if x, y, w, h := AreaBox(*f.Grid, f, f.Grid.Areas[2]); x != cols[8].Start || w != 4*90+3*24 || y != rows[1].Start || h <= 0 {
		t.Fatalf("aside at %g,%g %g×%g", x, y, w, h)
	}
	if col, row := CellAt(*f.Grid, f, 700, 400); col != 6 || row != 3 {
		t.Fatalf("cell %d,%d", col, row)
	}
	bad := &proto.CanvasGrid{Cols: 12, Rows: 2, Areas: []proto.GridArea{{Name: "x", Col: 10, Row: 1, ColSpan: 4}}}
	if _, _, _, err := s.Edit(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpFrameGrid, Grid: bad}); err == nil {
		t.Fatal("an area past the grid")
	}
	if a, err := ParseArea("nav:1,2,3"); err != nil || a.ColSpan != 3 || a.RowSpan != 1 {
		t.Fatalf("parse %+v %v", a, err)
	}
}
