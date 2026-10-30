package unslop

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The checks: each reads the design and says what it found of one tell.

type check func(p page) (proto.UnslopFinding, bool)

var checks = []check{
	fakeData, unlabelledDeltas,
	centeredEverything, cardsInCards, equalCards, eyebrowPills,
	offGridSpacing, tooManySpacings,
	defaultFonts, heavyWeights, tooManySizes, proportionalNumbers,
	purpleGradients, decorativeGradients, gradientText, tooManyAccents, pureBlackWhite,
	glows, glass, sideStripes, pills, radiusChaos, borderAndShadow, shadowSprawl,
	emojiIcons,
	exclamations, hypeWords, greetings, genericButtons, emDashes, aiCopy,
	rainbowCharts, sampleData,
	gratuitousMotion, loops,
	lowContrast, focusRemoved, tinyText,
}

func finding(pass, rule, severity, title, fix string, examples []string, targets ...string) proto.UnslopFinding {
	var at []string
	for _, t := range unique(targets) {
		if t != "" {
			at = append(at, t)
		}
	}
	return proto.UnslopFinding{Rule: rule, Pass: passName(pass), Severity: severity, Title: title, Count: len(examples), Examples: unique(examples), Fix: fix, Targets: at}
}

func unique(list []string) []string {
	var out []string
	for _, s := range list {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// where names a rule for an example: its selector, or "style=" inline.
func (r rule) where() string {
	if r.selector == "" {
		return "style attribute"
	}
	s := strings.Join(strings.Fields(r.selector), " ")
	if len(s) > 40 {
		s = s[:40] + "…"
	}
	return s
}

var pxRe = regexp.MustCompile(`(-?[0-9]*\.?[0-9]+)px`)

func pxValues(v string) []float64 {
	var out []float64
	for _, m := range pxRe.FindAllStringSubmatch(v, -1) {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			out = append(out, f)
		}
	}
	return out
}

func spacingProp(k string) bool {
	return strings.HasPrefix(k, "margin") || strings.HasPrefix(k, "padding") || k == "gap" || k == "row-gap" || k == "column-gap"
}

// Spacing

// offGridSpacing finds spacings off the 4 px grid: half steps (6, 10, 14)
// weigh little, being often chosen; arbitrary ones (7, 13, 22) weigh most.
func offGridSpacing(p page) (proto.UnslopFinding, bool) {
	var ex, half, at, halfAt []string
	for _, r := range p.rules {
		for k, v := range r.decls {
			if !spacingProp(k) {
				continue
			}
			for _, n := range pxValues(v) {
				a := math.Abs(n)
				switch {
				case a <= 2 || math.Mod(a, 4) == 0:
				case math.Mod(a, 2) == 0:
					half = append(half, fmt.Sprintf("%s %s: %gpx", r.where(), k, n))
					halfAt = append(halfAt, r.selectable())
				default:
					ex = append(ex, fmt.Sprintf("%s %s: %gpx", r.where(), k, n))
					at = append(at, r.selectable())
				}
			}
		}
	}
	sev := proto.UnslopMedium
	switch {
	case len(ex) >= 6:
		sev = proto.UnslopHigh
	case len(ex) >= 2:
	case len(half) >= 6:
		sev, ex, at = proto.UnslopLow, half, halfAt
	default:
		return proto.UnslopFinding{}, false
	}
	if sev != proto.UnslopLow {
		ex, at = append(ex, half...), append(at, halfAt...)
	}
	return finding("spacing", "spacing-off-grid", sev, "Spacing off a 4 px grid",
		"Put every margin, padding and gap on one scale (multiples of 4); no value picked by eye.", ex, at...), true
}

func tooManySpacings(p page) (proto.UnslopFinding, bool) {
	seen := map[float64]bool{}
	for _, r := range p.rules {
		for k, v := range r.decls {
			if spacingProp(k) {
				for _, n := range pxValues(v) {
					if n > 0 {
						seen[n] = true
					}
				}
			}
		}
	}
	if len(seen) <= 10 {
		return proto.UnslopFinding{}, false
	}
	var ex []string
	for n := range seen {
		ex = append(ex, fmt.Sprintf("%gpx", n))
	}
	slices.Sort(ex)
	f := finding("spacing", "spacing-scale", proto.UnslopLow, fmt.Sprintf("%d different spacings, no scale", len(seen)),
		"Use few spacing values: the same distance for the same relation.", ex)
	f.Count = len(seen)
	return f, true
}

// Structure

func centeredEverything(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	aligned := 0
	for _, r := range p.rules {
		if v, ok := r.decls["text-align"]; ok {
			aligned++
			if v == "center" {
				ex = append(ex, r.where())
				at = append(at, r.selectable())
			}
		}
	}
	if len(ex) < 5 || len(ex)*10 < aligned*6 {
		return proto.UnslopFinding{}, false
	}
	return finding("structure", "centered", proto.UnslopLow, "Text centered almost everywhere",
		"Align text left in cards, lists and forms; center only a short hero or an empty state.", ex, at...), true
}

// Color

func gradientRules(p page) []rule {
	var out []rule
	for _, r := range p.rules {
		for k, v := range r.decls {
			if strings.Contains(v, "gradient(") && (strings.HasPrefix(k, "background") || k == "border-image" || k == "color") {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

func purpleGradients(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range gradientRules(p) {
		for k, v := range r.decls {
			if !strings.Contains(v, "gradient(") {
				continue
			}
			n := 0
			for _, c := range colorsIn(v) {
				if c.purplish() {
					n++
				}
			}
			if n > 0 {
				ex = append(ex, r.where()+" "+k)
				at = append(at, r.selectable())
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("color", "purple-gradient", proto.UnslopHigh, "Violet / indigo / pink gradients",
		"Drop the decorative gradients; keep the design's own color direction, applied flat.", ex, at...), true
}

func decorativeGradients(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range gradientRules(p) {
		ex = append(ex, r.where())
		at = append(at, r.selectable())
	}
	if len(ex) < 3 {
		return proto.UnslopFinding{}, false
	}
	return finding("color", "gradients", proto.UnslopMedium, "Gradients as decoration",
		"Keep only a gradient that means something; flatten the rest.", ex, at...), true
}

func gradientText(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		if r.decls["background-clip"] == "text" || r.decls["-webkit-background-clip"] == "text" {
			ex = append(ex, r.where())
			at = append(at, r.selectable())
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("color", "gradient-text", proto.UnslopHigh, "Gradient text",
		"Set headings in the text color; weight and size make them stand out.", ex, at...), true
}

func glows(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		for _, k := range []string{"box-shadow", "text-shadow", "filter"} {
			v, ok := r.decls[k]
			if !ok || (k == "filter" && !strings.Contains(v, "drop-shadow")) {
				continue
			}
			vivid := slices.ContainsFunc(colorsIn(v), rgb.vivid)
			blur := slices.ContainsFunc(pxValues(v), func(n float64) bool { return n >= 12 })
			if vivid && blur {
				ex = append(ex, r.where()+" "+k)
				at = append(at, r.selectable())
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("surfaces", "glow", proto.UnslopMedium, "Colored glows",
		"Drop colored glows; a surface that needs lifting gets the one elevation used the same way everywhere.", ex, at...), true
}

func tooManyAccents(p page) (proto.UnslopFinding, bool) {
	hues := map[int]string{}
	for _, r := range p.rules {
		for k, v := range r.decls {
			if strings.Contains(k, "shadow") {
				continue
			}
			for _, c := range colorsIn(v) {
				if c.vivid() {
					h, _, _ := c.hsl()
					if _, ok := hues[int(h)/30]; !ok {
						hues[int(h)/30] = r.where()
					}
				}
			}
		}
	}
	if len(hues) < 5 {
		return proto.UnslopFinding{}, false
	}
	var ex []string
	for _, w := range hues {
		ex = append(ex, w)
	}
	slices.Sort(ex)
	f := finding("color", "accents", proto.UnslopMedium, fmt.Sprintf("%d accent hues", len(hues)),
		"Keep only accents that mean something (an action, a state); what only decorates stays out of color.", ex)
	f.Count = len(hues)
	return f, true
}

func glass(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		for _, k := range []string{"backdrop-filter", "-webkit-backdrop-filter"} {
			if strings.Contains(r.decls[k], "blur") {
				ex = append(ex, r.where())
				at = append(at, r.selectable())
				break
			}
		}
	}
	if len(ex) < 2 {
		return proto.UnslopFinding{}, false
	}
	return finding("surfaces", "glass", proto.UnslopLow, "Frosted glass everywhere",
		"Use solid surfaces; keep blur for an overlay over moving content, if any.", ex, at...), true
}

// Icons

func isEmoji(r rune) bool {
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF, r >= 0x1F000 && r <= 0x1F2FF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return !strings.ContainsRune("✓✔✗✘➜➔➝➞→", r)
	case r == 0x2B50, r == 0x2B55, r == 0x2B06, r == 0x2B07, r == 0x2B05, r == 0x2B1B, r == 0x2B1C:
		return true
	}
	return false
}

func emojiIcons(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		t := tt.s
		for _, r := range t {
			if isEmoji(r) {
				ex = append(ex, string(r)+" in “"+short(t)+"”")
				at = append(at, tt.target)
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	sev := proto.UnslopMedium
	if len(ex) >= 3 {
		sev = proto.UnslopHigh
	}
	return finding("icons", "emoji", sev, "Emoji as icons or decoration",
		"Replace each with an icon of one set, drawn at one stroke in currentColor, or drop it from labels and copy.", ex, at...), true
}

func short(s string) string {
	r := []rune(s)
	if len(r) > 32 {
		return string(r[:32]) + "…"
	}
	return s
}
