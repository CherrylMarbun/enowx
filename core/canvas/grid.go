package canvas

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

// A design's layout grid is CSS grid's: columns and rows of equal
// fractions, gaps between them, a margin around them, and areas spanning
// cells. The page shows it as guides, shapes snap to its lines, and its
// CSS (and a skeleton of its areas) is what a mockup is laid out with, so
// designs sit on one grid with even spacing.

var areaName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// CheckGrid checks g for frame f and fills in what it leaves out.
func CheckGrid(g *proto.CanvasGrid, f proto.CanvasFrame) error {
	if g.Cols < 1 || g.Cols > 48 {
		return errors.New("canvas: a grid has 1 to 48 columns")
	}
	if g.Rows < 1 || g.Rows > 48 {
		return errors.New("canvas: a grid has 1 to 48 rows")
	}
	if g.ColGap < 0 || g.RowGap < 0 || g.Margin < 0 {
		return errors.New("canvas: a grid's gaps and margin are 0 or more")
	}
	inner := float64(f.Width) - 2*g.Margin - float64(g.Cols-1)*g.ColGap
	if inner < float64(g.Cols) {
		return fmt.Errorf("canvas: %d columns with those gaps don't fit %d wide", g.Cols, f.Width)
	}
	seen := map[string]bool{}
	for i := range g.Areas {
		a := &g.Areas[i]
		a.Name = strings.ToLower(strings.TrimSpace(a.Name))
		if !areaName.MatchString(a.Name) {
			return fmt.Errorf("canvas: area %q: name it in lowercase letters, digits and dashes (header, main, side-nav)", a.Name)
		}
		if seen[a.Name] {
			return fmt.Errorf("canvas: two areas are named %q", a.Name)
		}
		seen[a.Name] = true
		a.ColSpan, a.RowSpan = max(a.ColSpan, 1), max(a.RowSpan, 1)
		if a.Col < 1 || a.Row < 1 || a.Col+a.ColSpan-1 > g.Cols || a.Row+a.RowSpan-1 > g.Rows {
			return fmt.Errorf("canvas: area %s goes past the grid's %d×%d cells", a.Name, g.Cols, g.Rows)
		}
	}
	return nil
}

// Track is a column's or a row's start and size, in the frame.
type Track struct{ Start, Size float64 }

// Tracks are g's columns and rows in frame f, f's height shared by the
// rows.
func Tracks(g proto.CanvasGrid, f proto.CanvasFrame) (cols, rows []Track) {
	split := func(length float64, n int, gap float64) []Track {
		size := (length - 2*g.Margin - float64(n-1)*gap) / float64(n)
		out := make([]Track, n)
		for i := range out {
			out[i] = Track{Start: g.Margin + float64(i)*(size+gap), Size: size}
		}
		return out
	}
	return split(float64(f.Width), max(g.Cols, 1), g.ColGap), split(float64(f.Height), max(g.Rows, 1), g.RowGap)
}

// AreaBox is where area a lies in frame f.
func AreaBox(g proto.CanvasGrid, f proto.CanvasFrame, a proto.GridArea) (x, y, w, h float64) {
	cols, rows := Tracks(g, f)
	c0, c1 := cols[min(a.Col-1, len(cols)-1)], cols[min(a.Col+a.ColSpan-2, len(cols)-1)]
	r0, r1 := rows[min(a.Row-1, len(rows)-1)], rows[min(a.Row+a.RowSpan-2, len(rows)-1)]
	return c0.Start, r0.Start, c1.Start + c1.Size - c0.Start, r1.Start + r1.Size - r0.Start
}

// CellAt is the cell (from 1) at point (x, y) of frame f, the nearest.
func CellAt(g proto.CanvasGrid, f proto.CanvasFrame, x, y float64) (col, row int) {
	cols, rows := Tracks(g, f)
	near := func(tracks []Track, v float64) int {
		best, far := 0, math.Inf(1)
		for i, t := range tracks {
			d := math.Abs(v - (t.Start + t.Size/2))
			if d < far {
				best, far = i, d
			}
		}
		return best + 1
	}
	return near(cols, x), near(rows, y)
}

// Snap is box (x, y, w, h) on g's lines in frame f: its edges at the
// nearest columns' and rows' edges, a cell at least.
func Snap(g proto.CanvasGrid, f proto.CanvasFrame, x, y, w, h float64) (float64, float64, float64, float64) {
	cols, rows := Tracks(g, f)
	snap := func(tracks []Track, start, size float64) (float64, float64) {
		starts, ends := make([]float64, len(tracks)), make([]float64, len(tracks))
		for i, t := range tracks {
			starts[i], ends[i] = t.Start, t.Start+t.Size
		}
		nearest := func(list []float64, v float64) int {
			best := 0
			for i := range list {
				if math.Abs(list[i]-v) < math.Abs(list[best]-v) {
					best = i
				}
			}
			return best
		}
		i := nearest(starts, start)
		j := max(nearest(ends, start+size), i)
		return starts[i], ends[j] - starts[i]
	}
	x, w = snap(cols, x, w)
	y, h = snap(rows, y, h)
	return x, y, w, h
}

// GridCSS is g's CSS: the container (.grid, in the frame) and each area by
// its id, to lay a mockup out on it.
func GridCSS(g proto.CanvasGrid) string {
	var b strings.Builder
	fmt.Fprintf(&b, ".grid {\n  display: grid;\n  grid-template-columns: repeat(%d, minmax(0, 1fr));\n  grid-template-rows: repeat(%d, minmax(0, 1fr));\n", g.Cols, g.Rows)
	fmt.Fprintf(&b, "  column-gap: %gpx;\n  row-gap: %gpx;\n", g.ColGap, g.RowGap)
	if g.Margin > 0 {
		fmt.Fprintf(&b, "  padding: %gpx;\n", g.Margin)
	}
	b.WriteString("  box-sizing: border-box;\n  min-height: 100vh;\n}\n")
	for _, a := range g.Areas {
		// row-start / column-start / row-end / column-end
		fmt.Fprintf(&b, "#%s { grid-area: %d / %d / %d / %d; }\n", a.Name, a.Row, a.Col, a.Row+a.RowSpan, a.Col+a.ColSpan)
	}
	return b.String()
}

// GridHTML is a mockup laid out on g: its CSS and an element for each area,
// empty, for agents and the user to fill an area at a time.
func GridHTML(g proto.CanvasGrid) string {
	var b strings.Builder
	b.WriteString("<style>\n" + GridCSS(g) + "</style>\n<main id=\"grid\" class=\"grid\">\n")
	for _, a := range g.Areas {
		fmt.Fprintf(&b, "  <section id=\"%s\"></section>\n", a.Name)
	}
	b.WriteString("</main>\n")
	return b.String()
}

// ParseArea reads an area as NAME:COL,ROW,COLSPAN,ROWSPAN (spans 1 when
// left out).
func ParseArea(s string) (proto.GridArea, error) {
	name, cells, ok := strings.Cut(s, ":")
	if !ok {
		return proto.GridArea{}, fmt.Errorf("area %q: write NAME:COL,ROW,COLSPAN,ROWSPAN, as header:1,1,12,1", s)
	}
	a := proto.GridArea{Name: name, ColSpan: 1, RowSpan: 1}
	n, err := fmt.Sscanf(cells, "%d,%d,%d,%d", &a.Col, &a.Row, &a.ColSpan, &a.RowSpan)
	if err != nil && n < 2 {
		return proto.GridArea{}, fmt.Errorf("area %q: write NAME:COL,ROW,COLSPAN,ROWSPAN, as header:1,1,12,1", s)
	}
	return a, nil
}

// snapShape puts el on f's grid when it snaps: shapes, not lines.
func snapShape(f proto.CanvasFrame, el *proto.CanvasElement) {
	if f.Grid == nil || !f.Grid.Snap || el.Type == proto.ShapeLine || el.Type == proto.ShapeArrow {
		return
	}
	el.X, el.Y, el.W, el.H = Snap(*f.Grid, f, el.X, el.Y, el.W, el.H)
}
