package ui

import (
	"math"
	"testing"
)

// The fan's children are sub-slices of the open wedge, cut from the pie's own
// annulus and slid out past the rim: they sit clear of the centre and outside
// the ring, and stay inside the wedge they came from.
func TestFloatingFanKeepsClear(t *testing.T) {
	app, tt := floatingWithSessions(t)
	if err := tt.Click("Active sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(floatingBloomTime)
	hx, hy := floatingCenter(t, tt, "Hide the floating control")
	ring := app.floatingActions()
	centres := map[string][2]float32{}
	for _, act := range ring {
		x, y := floatingCenter(t, tt, act.label)
		centres[act.key] = [2]float32{x, y}
	}
	pie := floatingSlicesFor(floatingWedges)
	at := 0
	for i, act := range ring {
		if act.fan == floatingActiveFan {
			at = i
		}
	}
	subs := floatingFanSlices(pie, at, len(app.floatingSources()), 1)
	parent := pie[at]
	for i, act := range app.floatingSources() {
		x, y := floatingCenter(t, tt, act.label)
		if x < 0 || y < 0 || x > floatingSize || y > floatingSize {
			t.Fatalf("%q sits off the canvas: %v, %v", act.label, x, y)
		}
		if d := math.Hypot(float64(x-hx), float64(y-hy)); d < (floatingHubSize+2*floatingFanTarget)/2 {
			t.Fatalf("%q covers the centre, %0.f DIPs from it", act.label, d)
		}
		// The child's own sub-slice is inside its wedge, and its icon sits in
		// that sub-slice's middle, out past the ring.
		if s := subs[i]; s.Start < parent.Start-1e-9 || s.End > parent.End+1e-9 {
			t.Fatalf("%q's sub-slice %+v leaves the wedge %+v", act.label, s, parent)
		}
		angle := subs[i].Mid()
		got := math.Atan2(float64(x-floatingSize/2), -float64(y-floatingSize/2))
		if d := math.Abs(floatingWrap(angle - got)); d > 0.02 {
			t.Fatalf("%q sits %.3f rad off its sub-slice", act.label, d)
		}
		r := math.Hypot(float64(x-floatingSize/2), float64(y-floatingSize/2))
		if want := float64(floatingFanIn+floatingFanOut)/2 + floatingFanLift; math.Abs(r-want) > 2 {
			t.Fatalf("%q sits %0.f DIPs out, want the slid fan annulus's middle %0.f", act.label, r, want)
		}
		for _, other := range ring {
			// A child comes out of its own wedge, so it may sit near that
			// wedge's icon; every other slice's must stay clear.
			if other.key == floatingActiveFan {
				continue
			}
			at := centres[other.key]
			want := float64(floatingSliceTarget+floatingFanTarget) / 2
			if d := math.Hypot(float64(x-at[0]), float64(y-at[1])); d < want {
				t.Fatalf("%q sits %0.f DIPs from %q, want at least %0.f", act.label, d, other.label, want)
			}
		}
	}
}

// The fan never overlaps the pie: every child's inner radius is past the pie's
// outer radius, at every point of the bloom. That is the whole point of the
// fan sitting above the rim rather than inside the wedge.
func TestFloatingFanAboveThePie(t *testing.T) {
	for _, f := range []float32{0, 0.25, 0.5, 1} {
		inner := float32(floatingFanIn) + floatingFanLift*f
		if inner <= floatingPizzaOut {
			t.Fatalf("at f=%v the fan's inner edge is %v, inside the pie's rim %d", f, inner, floatingPizzaOut)
		}
	}
	// And the drawn sectors use exactly that annulus: chart.Sector's path
	// stays between its two radii, so a sector built with them cannot reach
	// the pie.
	pie := floatingSlicesFor(floatingWedges)
	subs := floatingFanSlices(pie, 0, len(floatingSourcesForTest()), 1)
	inner := float32(floatingFanIn) + floatingFanLift
	outer := float32(floatingFanOut) + floatingFanLift
	for i, s := range subs {
		if s.End <= s.Start {
			t.Fatalf("sub-slice %d is empty: %+v", i, s)
		}
	}
	if inner <= floatingPizzaOut+floatingDecoratorGap+floatingDecoratorWidth {
		t.Fatalf("the fan starts at %v, over the decorator ring at %d",
			inner, floatingPizzaOut+floatingDecoratorGap+floatingDecoratorWidth)
	}
	if outer <= inner {
		t.Fatalf("the fan's annulus is %v..%v", inner, outer)
	}
	// It fits the canvas: the window is only as big as the pie's own reach,
	// and every DIP past that is transparent window that eats clicks.
	if want := 2 * (float32(outer) + 2); float32(floatingSize) < want {
		t.Fatalf("the canvas is %d, too small for a fan reaching %v: needs %v", floatingSize, outer, want)
	}
}

// floatingSourcesForTest is the fan's children, without an App.
func floatingSourcesForTest() []floatingAction {
	return []floatingAction{{key: "source:pc"}, {key: "source:vps"}}
}

// The fan's sub-slices collapse onto the wedge's mid angle as the bloom
// starts, and grow to fill it: one slice each, a floatingFanPad between them.
func TestFloatingFanSlices(t *testing.T) {
	pie := floatingSlicesFor(floatingWedges)
	parent := pie[0]
	mid := parent.Mid()
	for _, n := range []int{2, 3} {
		collapsed := floatingFanSlices(pie, 0, n, 0)
		if len(collapsed) != n {
			t.Fatalf("%d children cut %d sub-slices", n, len(collapsed))
		}
		for i, s := range collapsed {
			if math.Abs(s.Start-mid) > 1e-9 || math.Abs(s.End-mid) > 1e-9 {
				t.Fatalf("sub-slice %d of %d starts at %+v, want the wedge's mid %.3f", i, n, s, mid)
			}
		}
		open := floatingFanSlices(pie, 0, n, 1)
		from := parent.Start + parent.Pad/2
		to := parent.End - parent.Pad/2
		if got := open[0].Start - from; math.Abs(got) > 1e-9 {
			t.Fatalf("the first sub-slice starts %v in, want the wedge's own edge", got)
		}
		if got := to - open[n-1].End; math.Abs(got) > 1e-9 {
			t.Fatalf("the last sub-slice ends %v short, want the wedge's own edge", got)
		}
		for i, s := range open {
			if s.Pad != floatingFanPad {
				t.Fatalf("sub-slice %d pads %v, want %v", i, s.Pad, floatingFanPad)
			}
			if i > 0 && s.Start < open[i-1].End {
				t.Fatalf("sub-slice %d starts at %v, before %d ends at %v", i, s.Start, i-1, open[i-1].End)
			}
		}
	}
	// The gap is the pie's own width in DIPs, not its angle: a fan child is
	// further out, where the same angle is a wider cut.
	pieGap := floatingPizzaPad * math.Sqrt(float64(floatingPizzaIn*floatingPizzaIn+floatingPizzaOut*floatingPizzaOut))
	fanGap := floatingFanPad * math.Sqrt(float64(floatingFanIn*floatingFanIn+floatingFanOut*floatingFanOut))
	if math.Abs(fanGap-pieGap) > 0.5 {
		t.Fatalf("a fan's gap is %0.2f DIP, want the pie's %0.2f", fanGap, pieGap)
	}
	// Halfway through the bloom, the sub-slices cover half the wedge's span.
	half := floatingFanSlices(pie, 0, 2, 0.5)
	open := floatingFanSlices(pie, 0, 2, 1)
	if got, want := half[1].End-mid, (open[1].End-mid)/2; math.Abs(got-want) > 1e-9 {
		t.Fatalf("at half the bloom the span is %v, want %v", got, want)
	}
}
