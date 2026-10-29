package unslop

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// rgb is a color of a style, its channels 0–1.
type rgb struct{ r, g, b, a float64 }

var colorRe = regexp.MustCompile(`#[0-9a-f]{3,8}\b|rgba?\([^)]*\)|hsla?\([^)]*\)`)

// colorsIn are the colors a CSS value names, in order.
func colorsIn(v string) []rgb {
	var out []rgb
	for _, m := range colorRe.FindAllString(v, -1) {
		if c, ok := parseColor(m); ok {
			out = append(out, c)
		}
	}
	return out
}

func parseColor(s string) (rgb, bool) {
	switch {
	case strings.HasPrefix(s, "#"):
		h := s[1:]
		if len(h) == 3 || len(h) == 4 {
			var b strings.Builder
			for _, c := range h {
				b.WriteRune(c)
				b.WriteRune(c)
			}
			h = b.String()
		}
		if len(h) != 6 && len(h) != 8 {
			return rgb{}, false
		}
		n, err := strconv.ParseUint(h, 16, 64)
		if err != nil {
			return rgb{}, false
		}
		if len(h) == 6 {
			n = n<<8 | 0xff
		}
		return rgb{float64(n>>24&0xff) / 255, float64(n>>16&0xff) / 255, float64(n>>8&0xff) / 255, float64(n&0xff) / 255}, true
	case strings.HasPrefix(s, "rgb"):
		v := numbers(s)
		if len(v) < 3 {
			return rgb{}, false
		}
		c := rgb{v[0] / 255, v[1] / 255, v[2] / 255, 1}
		if len(v) > 3 {
			c.a = alpha(v[3], s)
		}
		return c, true
	case strings.HasPrefix(s, "hsl"):
		v := numbers(s)
		if len(v) < 3 {
			return rgb{}, false
		}
		c := fromHSL(v[0], v[1]/100, v[2]/100)
		if len(v) > 3 {
			c.a = alpha(v[3], s)
		}
		return c, true
	}
	return rgb{}, false
}

var numberRe = regexp.MustCompile(`-?[0-9]*\.?[0-9]+%?`)

func numbers(s string) []float64 {
	var out []float64
	for _, m := range numberRe.FindAllString(s, -1) {
		f, err := strconv.ParseFloat(strings.TrimSuffix(m, "%"), 64)
		if err == nil {
			out = append(out, f)
		}
	}
	return out
}

func alpha(v float64, s string) float64 {
	if v > 1 || strings.Contains(s[strings.LastIndexAny(s, ",/")+1:], "%") {
		return v / 100
	}
	return v
}

func fromHSL(h, s, l float64) rgb {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	if s == 0 {
		return rgb{l, l, l, 1}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	f := func(t float64) float64 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return rgb{f(h + 1.0/3), f(h), f(h - 1.0/3), 1}
}

// hsl is c's hue (degrees), saturation and lightness.
func (c rgb) hsl() (h, s, l float64) {
	mx, mn := math.Max(c.r, math.Max(c.g, c.b)), math.Min(c.r, math.Min(c.g, c.b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case c.r:
		h = math.Mod((c.g-c.b)/d+6, 6)
	case c.g:
		h = (c.b-c.r)/d + 2
	default:
		h = (c.r-c.g)/d + 4
	}
	return h * 60, s, l
}

// vivid says c is a strong color, not a gray, near black or white.
func (c rgb) vivid() bool {
	_, s, l := c.hsl()
	return s > 0.45 && l > 0.25 && l < 0.8 && c.a > 0.3
}

// purplish says c is in the violet, indigo and magenta hues AI tools reach
// for.
func (c rgb) purplish() bool {
	h, _, _ := c.hsl()
	return c.vivid() && h >= 245 && h <= 330
}

func (c rgb) luminance() float64 {
	ch := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.r) + 0.7152*ch(c.g) + 0.0722*ch(c.b)
}

// contrast is the WCAG contrast ratio of a and b.
func contrast(a, b rgb) float64 {
	la, lb := a.luminance(), b.luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
