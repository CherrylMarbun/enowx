package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// sliceAt reports whether the pointer is in slice i of pie: chart.SliceAt
// answers which slice it is over, with the ring's radii widened so a pointer
// just past the rim still selects, as a pie menu must.
func (a *App) sliceAt(panel ui.Element, pie []chart.Slice, i int) bool {
	x, y, over := panel.PointerPosition()
	if !over {
		return false
	}
	cx, cy := float32(floatingSize)/2, float32(floatingSize)/2
	inner := float32(floatingPizzaIn) - floatingSelectIn*floatingRingWidth
	outer := float32(floatingPizzaOut) + floatingSelectOut*floatingRingWidth
	j, ok := chart.SliceAt(pie, x-cx, y-cy, inner, outer)
	return ok && j == i
}

// floatingSlicesFor is the pie of n equal slices, covering floatingCoverage
// of the circle from floatingCenterAngle: a full pie from twelve o'clock for
// seven actions.
func floatingSlicesFor(n int) []chart.Slice {
	shares := make([]float64, n)
	for i := range shares {
		shares[i] = 1
	}
	out := chart.Pie(shares, floatingPizzaPad)
	// chart's first slice begins at twelve o'clock, its middle half a slice
	// later: turning the pie back by that half puts the first slice's own
	// middle at the top, which is where the first action belongs.
	base := floatingCenterAngle + math.Pi/2 - floatingStep*floatingCoverage/2
	for i := range out {
		out[i].Start = base + out[i].Start*floatingCoverage
		out[i].End = base + out[i].End*floatingCoverage
	}
	return out
}

// floatingMidOf is where slice i of pie points, in the panel's convention:
// anticlockwise from the right, with the first at the top.
func floatingMidOf(pie []chart.Slice, i int) float64 {
	if i < len(pie) {
		return math.Pi/2 - pie[i].Mid()
	}
	return 0
}

// floatingWrap brings an angle to the half turn each side of zero.
func floatingWrap(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
