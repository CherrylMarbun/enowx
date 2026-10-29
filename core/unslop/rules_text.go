package unslop

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The checks of type, copy, components, motion and contrast.

// Type

// firstFamily is the first family a font or font-family value names.
func firstFamily(k, v string) string {
	if k == "font" {
		// The family follows the size: "700 18px/1.4 Arial, sans-serif".
		i := pxRe.FindStringIndex(v)
		if i == nil {
			return ""
		}
		v = v[i[1]:]
		if strings.HasPrefix(v, "/") {
			if j := strings.IndexByte(v, ' '); j >= 0 {
				v = v[j:]
			}
		}
	}
	f, _, _ := strings.Cut(v, ",")
	return strings.Trim(strings.TrimSpace(f), `"'`)
}

var defaultFamilies = []string{"arial", "helvetica", "helvetica neue", "times", "times new roman", "sans-serif", "serif", "system-ui", "-apple-system", "verdana", "roboto"}

// newDefaults are the faces AI tools now reach for first: fine with a
// reason, a tell as everything's display face.
var newDefaults = []string{"inter", "geist", "space grotesk", "instrument serif", "poppins", "dm sans", "plus jakarta sans"}

func defaultFonts(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		for _, k := range []string{"font-family", "font"} {
			if v, ok := r.decls[k]; ok && slices.Contains(defaultFamilies, firstFamily(k, v)) {
				ex = append(ex, r.where()+": "+firstFamily(k, v))
				at = append(at, r.selectable())
			}
		}
	}
	if len(ex) == 0 {
		for _, r := range p.rules {
			for _, k := range []string{"font-family", "font"} {
				if v, ok := r.decls[k]; ok && slices.Contains(newDefaults, firstFamily(k, v)) {
					ex = append(ex, r.where()+": "+firstFamily(k, v))
					at = append(at, r.selectable())
				}
			}
		}
		if len(ex) == 0 {
			return proto.UnslopFinding{}, false
		}
		return finding("type", "default-font", proto.UnslopLow, "Today's default typeface",
			"Keep it only with a reason; otherwise choose faces for the product's character (a display and a text face).", ex, at...), true
	}
	return finding("type", "default-font", proto.UnslopMedium, "A default typeface",
		"Choose faces for the product's character, with a reason, not the first default (Inter, Geist, Space Grotesk, Instrument Serif are today's); load them from Google Fonts or a CDN.", ex, at...), true
}

var weightRe = regexp.MustCompile(`\b(bold|bolder|[1-9]00)\b`)

func heavyWeights(p page) (proto.UnslopFinding, bool) {
	heavy, all := 0, 0
	var ex, at []string
	for _, r := range p.rules {
		for _, k := range []string{"font-weight", "font"} {
			v, ok := r.decls[k]
			if !ok {
				continue
			}
			w := weightRe.FindString(v)
			if w == "" {
				continue
			}
			all++
			if w == "bold" || w == "bolder" || w >= "700" {
				heavy++
				ex = append(ex, r.where()+": "+w)
				at = append(at, r.selectable())
			}
		}
	}
	if heavy < 4 || heavy*2 <= all {
		return proto.UnslopFinding{}, false
	}
	f := finding("type", "bold-everywhere", proto.UnslopMedium, fmt.Sprintf("Bold on %d of %d weights", heavy, all),
		"Use regular (400) for text, medium (500) for labels, semibold (600) for the few headings; big numbers often read best light (300).", ex, at...)
	f.Count = heavy
	return f, true
}

func tooManySizes(p page) (proto.UnslopFinding, bool) {
	seen := map[float64]bool{}
	for _, r := range p.rules {
		if v, ok := r.decls["font-size"]; ok {
			for _, n := range pxValues(v) {
				seen[n] = true
			}
		}
		if v, ok := r.decls["font"]; ok {
			if n := pxValues(v); len(n) > 0 {
				seen[n[0]] = true
			}
		}
	}
	if len(seen) <= 9 {
		return proto.UnslopFinding{}, false
	}
	var ex []string
	for n := range seen {
		ex = append(ex, fmt.Sprintf("%gpx", n))
	}
	slices.Sort(ex)
	f := finding("type", "type-scale", proto.UnslopLow, fmt.Sprintf("%d font sizes, no scale", len(seen)),
		"Use a scale of six or seven sizes (12, 13, 14, 16, 20, 24, 32…).", ex)
	f.Count = len(seen)
	return f, true
}

var figureRe = regexp.MustCompile(`[$€£¥]?\d{1,3}([,.]\d{3})+([.,]\d+)?|\d+\.\d+`)

func proportionalNumbers(p page) (proto.UnslopFinding, bool) {
	for _, r := range p.rules {
		for _, v := range r.decls {
			if strings.Contains(v, "tabular-nums") || strings.Contains(v, `"tnum"`) {
				return proto.UnslopFinding{}, false
			}
		}
	}
	var ex, at []string
	for _, tt := range p.texts {
		t := tt.s
		if m := figureRe.FindString(t); m != "" {
			ex = append(ex, m)
			at = append(at, tt.target)
		}
	}
	if len(ex) < 3 {
		return proto.UnslopFinding{}, false
	}
	return finding("type", "tabular-numbers", proto.UnslopLow, "Figures without tabular numbers",
		"Set font-variant-numeric: tabular-nums on numbers and tables; make units and currency smaller and lighter than their numbers.", ex, at...), true
}

// Copy

func exclamations(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	double := false
	for _, tt := range p.texts {
		t := tt.s
		if strings.Contains(t, "!") {
			ex = append(ex, "“"+short(t)+"”")
			at = append(at, tt.target)
			double = double || strings.Contains(t, "!!")
		}
	}
	if len(ex) < 2 && !double {
		return proto.UnslopFinding{}, false
	}
	sev := proto.UnslopMedium
	if double || len(ex) >= 4 {
		sev = proto.UnslopHigh
	}
	return finding("copy", "exclamation", sev, "Exclamation marks",
		"State things plainly: “+12% vs last month”, not “+12%!!”.", ex, at...), true
}

var hype = []string{"supercharge", "seamless", "unlock", "effortless", "revolutionize", "revolutionary", "game-changer", "game changer",
	"next-level", "next level", "cutting-edge", "blazing", "elevate", "empower", "unleash", "magic", "magical", "10x", "ai-powered", "skyrocket", "boost your"}

func hypeWords(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		t := tt.s
		low := strings.ToLower(t)
		for _, w := range hype {
			if strings.Contains(low, w) {
				ex = append(ex, w+" in “"+short(t)+"”")
				at = append(at, tt.target)
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("copy", "hype", proto.UnslopMedium, "Hype words",
		"Say what it does, specifically: what changes for the user, with a number when there is one.", ex, at...), true
}

var greetingRe = regexp.MustCompile(`(?i)^(welcome back|hey|hi|hello|good (morning|afternoon|evening)|ready (for|to))\b`)

func greetings(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		t := tt.s
		if greetingRe.MatchString(t) {
			ex = append(ex, "“"+short(t)+"”")
			at = append(at, tt.target)
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("copy", "greeting", proto.UnslopLow, "A greeting where the page's point goes",
		"Lead with what the page is about (the date, the period, the status), not small talk.", ex, at...), true
}

// Components

func radii(p page) map[string][]float64 {
	out := map[string][]float64{}
	for _, r := range p.rules {
		for k, v := range r.decls {
			if strings.Contains(k, "radius") {
				out[r.where()] = append(out[r.where()], pxValues(v)...)
			}
		}
	}
	return out
}

func pills(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for w, vs := range radii(p) {
		if slices.ContainsFunc(vs, func(n float64) bool { return n >= 99 }) {
			ex = append(ex, w)
		}
	}
	if len(ex) < 6 {
		return proto.UnslopFinding{}, false
	}
	slices.Sort(ex)
	return finding("surfaces", "pills", proto.UnslopLow, "Pills and badges everywhere",
		"Keep badges for counts and states that need attention; plain text or a dot does the rest.", ex, at...), true
}

func radiusChaos(p page) (proto.UnslopFinding, bool) {
	seen := map[float64]bool{}
	big := 0
	for _, vs := range radii(p) {
		for _, n := range vs {
			if n > 0 && n < 99 {
				seen[n] = true
				if n >= 20 {
					big++
				}
			}
		}
	}
	if len(seen) < 6 && big < 3 {
		return proto.UnslopFinding{}, false
	}
	var ex, at []string
	for n := range seen {
		ex = append(ex, fmt.Sprintf("%gpx", n))
	}
	slices.Sort(ex)
	title := fmt.Sprintf("%d different radii", len(seen))
	if big >= 3 {
		title = "Large radii everywhere"
	}
	f := finding("surfaces", "radius", proto.UnslopLow, title,
		"Use two or three radii by size (6 for controls, 10–12 for cards, full for avatars); inner radius = outer minus padding.", ex, at...)
	f.Count = len(seen)
	return f, true
}

func borderAndShadow(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		b, s := r.decls["border"], r.decls["box-shadow"]
		if b != "" && b != "none" && b != "0" && s != "" && s != "none" {
			ex = append(ex, r.where())
			at = append(at, r.selectable())
		}
	}
	if len(ex) < 3 {
		return proto.UnslopFinding{}, false
	}
	return finding("surfaces", "border-shadow", proto.UnslopLow, "Borders and shadows together",
		"Separate surfaces with one: a hairline border on dark, a soft shadow on light.", ex, at...), true
}

// Motion

func gratuitousMotion(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, a := range p.atRules {
		if strings.HasPrefix(a, "@keyframes") {
			ex = append(ex, a)
		}
	}
	for _, r := range p.rules {
		if strings.Contains(r.selector, ":hover") && strings.Contains(r.decls["transform"], "scale") {
			ex = append(ex, r.where()+" scales")
			at = append(at, r.selectable())
		}
	}
	if len(ex) < 3 {
		return proto.UnslopFinding{}, false
	}
	return finding("motion", "motion", proto.UnslopLow, "Animation as decoration",
		"Remove entrance animations and hover growth; keep short transitions on state changes.", ex, at...), true
}

// Contrast

func lowContrast(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		fg := colorsIn(r.decls["color"])
		bgv := r.decls["background-color"]
		if bgv == "" && !strings.Contains(r.decls["background"], "gradient") {
			bgv = r.decls["background"]
		}
		bg := colorsIn(bgv)
		if len(fg) != 1 || len(bg) != 1 || fg[0].a < 0.99 || bg[0].a < 0.99 {
			continue
		}
		if c := contrast(fg[0], bg[0]); c < 4.5 {
			ex = append(ex, fmt.Sprintf("%s %.1f:1", r.where(), c))
			at = append(at, r.selectable())
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("access", "contrast", proto.UnslopHigh, "Text below 4.5:1 contrast",
		"Darken or lighten the text (or its background) until body text reaches 4.5:1 and large text 3:1.", ex, at...), true
}
