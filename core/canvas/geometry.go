package canvas

import (
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// ParseColor reads #rgb or #rrggbb.
func ParseColor(s string) (color.RGBA, bool) {
	s, ok := strings.CutPrefix(strings.TrimSpace(s), "#")
	if !ok {
		return color.RGBA{}, false
	}
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return color.RGBA{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.RGBA{}, false
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
}

// Style is how an element draws, its defaults filled in: fill and stroke
// ("" for none), text color and size.
type Style struct {
	Fill, Stroke, Color string
	StrokeWidth         float64
	FontSize            float64
}

// StyleOf is el's style, the shape's defaults where it says nothing.
func StyleOf(el proto.CanvasElement) Style {
	st := Style{Fill: el.Fill, Stroke: el.Stroke, Color: el.Color, StrokeWidth: el.StrokeWidth, FontSize: el.FontSize}
	switch el.Type {
	case proto.ShapeText:
		if st.Fill == "" {
			st.Fill = "none"
		}
		if st.Stroke == "" {
			st.Stroke = "none"
		}
	case proto.ShapeSticky:
		if st.Fill == "" {
			st.Fill = "#fff3b0"
		}
		if st.Stroke == "" {
			st.Stroke = "none"
		}
	case proto.ShapeLine, proto.ShapeArrow:
		st.Fill = "none"
		if st.Stroke == "" {
			st.Stroke = "#475569"
		}
	case proto.ShapeImage:
		if st.Stroke == "" {
			st.Stroke = "none"
		}
	default:
		if st.Fill == "" {
			st.Fill = "#ffffff"
		}
		if st.Stroke == "" {
			st.Stroke = "#94a3b8"
		}
	}
	if st.Fill == "none" {
		st.Fill = ""
	}
	if st.Stroke == "none" {
		st.Stroke = ""
	}
	if st.Color == "" {
		st.Color = "#0f172a"
	}
	if st.StrokeWidth <= 0 {
		st.StrokeWidth = 1.5
	}
	if st.FontSize <= 0 {
		st.FontSize = 16
	}
	return st
}

// Ends is where line or arrow el goes from and to: from the edge of
// element From towards To's, or (X, Y) to (X+W, Y+H).
func Ends(f proto.CanvasFrame, el proto.CanvasElement) (x1, y1, x2, y2 float64) {
	x1, y1, x2, y2 = el.X, el.Y, el.X+el.W, el.Y+el.H
	from, okF := byID(f, el.From)
	to, okT := byID(f, el.To)
	if okF {
		x1, y1 = from.X+from.W/2, from.Y+from.H/2
	}
	if okT {
		x2, y2 = to.X+to.W/2, to.Y+to.H/2
	}
	if okF {
		x1, y1 = edge(from, x2, y2)
	}
	if okT {
		x2, y2 = edge(to, x1, y1)
	}
	return
}

func byID(f proto.CanvasFrame, id string) (proto.CanvasElement, bool) {
	if id == "" {
		return proto.CanvasElement{}, false
	}
	i := slices.IndexFunc(f.Elements, func(e proto.CanvasElement) bool { return e.ID == id })
	if i < 0 {
		return proto.CanvasElement{}, false
	}
	return f.Elements[i], true
}

// edge is where the line from el's center to (x, y) leaves el's box.
func edge(el proto.CanvasElement, x, y float64) (float64, float64) {
	cx, cy := el.X+el.W/2, el.Y+el.H/2
	dx, dy := x-cx, y-cy
	if dx == 0 && dy == 0 {
		return cx, cy
	}
	t := math.Inf(1)
	if dx != 0 {
		t = math.Min(t, (el.W/2)/math.Abs(dx))
	}
	if dy != 0 {
		t = math.Min(t, (el.H/2)/math.Abs(dy))
	}
	if el.Type == proto.ShapeEllipse || el.Type == proto.ShapeDiamond {
		// Near enough for both: the ellipse's own edge.
		a, b := el.W/2, el.H/2
		if a > 0 && b > 0 {
			t = 1 / math.Sqrt(dx*dx/(a*a)+dy*dy/(b*b))
		}
	}
	return cx + dx*t, cy + dy*t
}

// ArrowHead is the two points of an arrow's head at (x2, y2), coming from
// (x1, y1).
func ArrowHead(x1, y1, x2, y2, size float64) (ax, ay, bx, by float64) {
	angle := math.Atan2(y2-y1, x2-x1)
	const spread = math.Pi / 7
	ax, ay = x2-size*math.Cos(angle-spread), y2-size*math.Sin(angle-spread)
	bx, by = x2-size*math.Cos(angle+spread), y2-size*math.Sin(angle+spread)
	return
}
