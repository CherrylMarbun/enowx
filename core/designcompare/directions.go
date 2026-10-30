// Package designcompare tells how designs differ: two directions of a
// build, by their cards and their first screens squinted at, and a
// recreated design against the picture it recreates.
package designcompare

import (
	"math"
	"strings"
	"unicode"

	"github.com/enowx/enowx/proto"
)

// MaxShared is how much of skeleton, type, palette and hero two
// directions may have in common.
const MaxShared = 1

// TooAlike is the squint likeness (0 to 100) past which two first screens
// read as one layout.
const TooAlike = 85

// Shared is what directions a and b have in common of their skeleton,
// type, palette and hero.
func Shared(a, b proto.DesignDirection) []string {
	var out []string
	if overlap(a.Skeleton, b.Skeleton) >= 0.5 {
		out = append(out, "skeleton")
	}
	if fa, fb := family(a.Type), family(b.Type); fa != "" && fa == fb {
		out = append(out, "type")
	}
	if samePalette(a.Palette, b.Palette) {
		out = append(out, "palette")
	}
	if overlap(a.Hero, b.Hero) >= 0.5 {
		out = append(out, "hero")
	}
	return out
}

// Pair is how directions a and b (designs ida and idb) differ, alike how
// alike their first screens are squinted at (-1 unknown).
func Pair(ida, idb string, a, b proto.DesignDirection, alike int) proto.DirectionPair {
	p := proto.DirectionPair{A: ida, B: idb, Shared: Shared(a, b), Alike: max(alike, 0)}
	d := 100 - 22*len(p.Shared)
	if alike >= 0 {
		d -= alike * 2 / 5
	}
	p.Distinct = min(max(d, 0), 100)
	return p
}

// TooClose says pair p fails: more in common than MaxShared, or first
// screens squinting alike.
func TooClose(p proto.DirectionPair) bool {
	return len(p.Shared) > MaxShared || p.Alike >= TooAlike
}

// words are s's words of three letters or more, lowercased.
func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len(w) >= 3 && !stop[w] {
			out[w] = true
		}
	}
	return out
}

var stop = map[string]bool{"the": true, "and": true, "with": true, "for": true, "its": true, "from": true, "one": true, "that": true, "are": true, "into": true}

// overlap is how much of the shorter of a and b's words the other has.
func overlap(a, b string) float64 {
	wa, wb := words(a), words(b)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	n := 0
	for w := range wa {
		if wb[w] {
			n++
		}
	}
	return float64(n) / float64(min(len(wa), len(wb)))
}

// family is the first font family a type line names, lowercased.
func family(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, sep := range []string{",", ";", "/", "+", " for ", " and ", "("} {
		s, _, _ = strings.Cut(s, sep)
	}
	var name []string
	for _, w := range strings.Fields(strings.Trim(s, `"'`)) {
		if strings.ContainsAny(w, "0123456789") || weights[w] {
			break
		}
		name = append(name, w)
	}
	return strings.Join(name, " ")
}

var weights = map[string]bool{"thin": true, "light": true, "regular": true, "medium": true, "semibold": true, "bold": true, "black": true, "italic": true}

// samePalette says two palettes share the background's lightness and the
// accent's hue family.
func samePalette(a, b []string) bool {
	if len(a) < 2 || len(b) < 2 {
		return false
	}
	ba, okA := hexColor(a[0])
	bb, okB := hexColor(b[0])
	aa, okC := hexColor(a[1])
	ab, okD := hexColor(b[1])
	if !okA || !okB || !okC || !okD {
		return false
	}
	_, _, la := ba.hsl()
	_, _, lb := bb.hsl()
	if (la < 0.5) != (lb < 0.5) {
		return false
	}
	ha, sa, _ := aa.hsl()
	hb, sb, _ := ab.hsl()
	if sa < 0.15 || sb < 0.15 {
		return sa < 0.15 && sb < 0.15 // both black and white
	}
	d := math.Abs(ha - hb)
	return min(d, 360-d) < 30
}

type rgb struct{ r, g, b float64 }

func hexColor(s string) (rgb, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return rgb{}, false
	}
	var v [3]float64
	for i := range 3 {
		n := 0
		for _, c := range s[i*2 : i*2+2] {
			switch {
			case c >= '0' && c <= '9':
				n = n*16 + int(c-'0')
			case c >= 'a' && c <= 'f':
				n = n*16 + int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				n = n*16 + int(c-'A') + 10
			default:
				return rgb{}, false
			}
		}
		v[i] = float64(n) / 255
	}
	return rgb{v[0], v[1], v[2]}, true
}

// hsl is c's hue (degrees), saturation and lightness.
func (c rgb) hsl() (h, s, l float64) {
	hi, lo := max(c.r, c.g, c.b), min(c.r, c.g, c.b)
	l = (hi + lo) / 2
	if hi == lo {
		return 0, 0, l
	}
	d := hi - lo
	s = d / (1 - math.Abs(2*l-1))
	switch hi {
	case c.r:
		h = math.Mod((c.g-c.b)/d, 6)
	case c.g:
		h = (c.b-c.r)/d + 2
	default:
		h = (c.r-c.g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}
