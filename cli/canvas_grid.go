package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/core/iconlib"
	"github.com/enowx/enowx/proto"
)

// A design's layout grid, and icons for mockups, from the command line.

const gridUsage = `layout grids and icons:
  grid CANVAS [--frame F] [--cols N --rows N --gap G | --col-gap G --row-gap G] [--margin M]
       [--area NAME:COL,ROW,COLSPAN,ROWSPAN]... [--no-snap] [--apply] [--off]
                                   a design's grid: shown as guides, shapes snap to it; prints its CSS.
                                   --apply lays the design's HTML out on it (an empty element per area)
  icon LIB:NAME [--size N]         an icon as inline SVG for a mockup (icons search finds names)`

var gridCommands = []string{"grid", "icon"}

func (r runner) canvasGrid(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	frame := fs.String("frame", "", "")
	cols := fs.Int("cols", 0, "")
	rows := fs.Int("rows", 0, "")
	gap := fs.Float64("gap", -1, "")
	colGap := fs.Float64("col-gap", -1, "")
	rowGap := fs.Float64("row-gap", -1, "")
	margin := fs.Float64("margin", -1, "")
	noSnap := fs.Bool("no-snap", false, "")
	apply := fs.Bool("apply", false, "")
	off := fs.Bool("off", false, "")
	size := fs.Int("size", 24, "")
	var areas []string
	fs.Func("area", "", func(v string) error { areas = append(areas, v); return nil })
	pos, err := parseAll(fs, args)
	if err != nil || len(pos) == 0 {
		return errors.New(gridUsage)
	}
	if cmd == "icon" {
		return r.canvasIcon(pos[0], *size)
	}
	c, err := r.findCanvas(pos[0])
	if err != nil {
		return err
	}
	i, err := canvas.FrameIndex(c, *frame)
	if err != nil {
		return err
	}
	f := c.Frames[i]
	edit := func(e proto.CanvasEdit) (proto.Canvas, error) {
		e.ID, e.Frame, e.Session = c.ID, f.ID, os.Getenv("ENOWX_SESSION")
		return r.c.EditCanvas(e)
	}
	if *off {
		_, err := edit(proto.CanvasEdit{Op: proto.CanvasOpFrameGrid})
		return err
	}
	changing := *cols > 0 || *rows > 0 || *gap >= 0 || *colGap >= 0 || *rowGap >= 0 || *margin >= 0 || len(areas) > 0 || isSet(fs, "no-snap")
	g := proto.CanvasGrid{Cols: 12, Rows: 6, ColGap: 24, RowGap: 24, Margin: 48, Snap: true}
	if f.Grid != nil {
		g = *f.Grid
	}
	if changing {
		if *cols > 0 {
			g.Cols = *cols
		}
		if *rows > 0 {
			g.Rows = *rows
		}
		if *gap >= 0 {
			g.ColGap, g.RowGap = *gap, *gap
		}
		if *colGap >= 0 {
			g.ColGap = *colGap
		}
		if *rowGap >= 0 {
			g.RowGap = *rowGap
		}
		if *margin >= 0 {
			g.Margin = *margin
		}
		g.Snap = !*noSnap
		if len(areas) > 0 {
			g.Areas = nil
			for _, s := range areas {
				a, err := canvas.ParseArea(s)
				if err != nil {
					return err
				}
				g.Areas = append(g.Areas, a)
			}
		}
		got, err := edit(proto.CanvasEdit{Op: proto.CanvasOpFrameGrid, Grid: &g})
		if err != nil {
			return err
		}
		f = got.Frames[i]
	} else if f.Grid == nil {
		fmt.Fprintf(r.out, "Design %d (%s) has no grid: --cols 12 --rows 6 --gap 24 --margin 48 gives it one.\n", i+1, f.Name)
		return nil
	}
	if *apply {
		if _, err := edit(proto.CanvasEdit{Op: proto.CanvasOpHTMLSet, HTML: canvas.GridHTML(*f.Grid)}); err != nil {
			return err
		}
	}
	gr := *f.Grid
	cs, rs := canvas.Tracks(gr, f)
	snap := "shapes snap to it"
	if !gr.Snap {
		snap = "shapes don't snap"
	}
	fmt.Fprintf(r.out, "Design %d (%s) %d×%d: %d columns of %.4g, %d rows of %.4g, gaps %g × %g, margin %g; %s\n",
		i+1, f.Name, f.Width, f.Height, gr.Cols, cs[0].Size, gr.Rows, rs[0].Size, gr.ColGap, gr.RowGap, gr.Margin, snap)
	for _, a := range gr.Areas {
		x, y, w, h := canvas.AreaBox(gr, f, a)
		fmt.Fprintf(r.out, "  #%s: columns %d–%d, rows %d–%d (at %g,%g %g×%g)\n", a.Name, a.Col, a.Col+a.ColSpan-1, a.Row, a.Row+a.RowSpan-1, x, y, w, h)
	}
	fmt.Fprintf(r.out, "\n%s", canvas.GridCSS(gr))
	if *apply {
		fmt.Fprintln(r.out, "\nThe design's HTML is laid out on it: fill each area's element (canvas patch --element NAME).")
	}
	return nil
}

// canvasIcon prints icon ref (lib:name) as inline SVG of size, its lines
// in the text's color.
func (r runner) canvasIcon(ref string, size int) error {
	lib, name, ok := strings.Cut(ref, ":")
	if !ok {
		return errors.New("usage: enowx-app canvas icon LIB:NAME (as lucide:rocket; enowx-app icons search finds names)")
	}
	svg, found := iconlib.Lookup(lib, name)
	if !found {
		return fmt.Errorf("no icon %s: enowx-app icons search %s", ref, name)
	}
	fmt.Fprintln(r.out, sizedSVG(svg, size))
	return nil
}

// sizedSVG is svg size wide and tall, its color the text's.
func sizedSVG(svg string, size int) string {
	open := strings.Index(svg, ">")
	if !strings.HasPrefix(svg, "<svg") || open < 0 {
		return svg
	}
	head := svg[:open]
	for _, attr := range []string{"width", "height"} {
		if i := strings.Index(head, " "+attr+"=\""); i >= 0 {
			if j := strings.Index(head[i+len(attr)+3:], "\""); j >= 0 {
				head = head[:i] + head[i+len(attr)+3+j+1:]
			}
		}
	}
	return fmt.Sprintf(`%s width="%d" height="%d" aria-hidden="true"%s`, head, size, size, svg[open:])
}
