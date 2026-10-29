package unslop

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The checks reading the element tree (cards in cards, rows of equal
// cards, eyebrow pills), charts, loops, focus, small text, pure black and
// white, shadows, and the copy's tics.

// blockTags are the elements a card is made of; a pill or badge (a span,
// a link) isn't one.
var blockTags = map[string]bool{"div": true, "section": true, "article": true, "aside": true, "li": true, "main": true,
	"header": true, "footer": true, "nav": true, "form": true, "figure": true, "details": true}

func cardsInCards(p page) (proto.UnslopFinding, bool) {
	card := make([]bool, len(p.nodes))
	for i, nd := range p.nodes {
		card[i] = blockTags[nd.tag] && cardLike(p.style(i))
	}
	var ex, at []string
	for i, nd := range p.nodes {
		if !card[i] {
			continue
		}
		for a := nd.parent; a >= 0; a = p.nodes[a].parent {
			if card[a] {
				ex = append(ex, p.nodes[a].target()+" › "+nd.target())
				at = append(at, nd.target())
				break
			}
		}
	}
	if len(ex) < 2 {
		return proto.UnslopFinding{}, false
	}
	return finding("structure", "cards-in-cards", proto.UnslopMedium, "Cards inside cards",
		"Let the inner parts sit on their card's surface: one level of card, grouped by space or a divider.", ex, at...), true
}

// equalCards finds a row of three or four cards alike, each led by an
// icon or picture: the template's feature row.
func equalCards(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, nd := range p.nodes {
		if len(nd.kids) < 3 || len(nd.kids) > 4 {
			continue
		}
		first := p.nodes[nd.kids[0]]
		alike := true
		for _, k := range nd.kids {
			kid := p.nodes[k]
			if kid.tag != first.tag || strings.Join(kid.classes, " ") != strings.Join(first.classes, " ") || !cardLike(p.style(k)) || len(kid.kids) == 0 {
				alike = false
				break
			}
			lead := p.nodes[kid.kids[0]]
			if lead.tag != "svg" && lead.tag != "img" && lead.tag != "i" && !(len(lead.kids) > 0 && p.nodes[lead.kids[0]].tag == "svg") {
				alike = false
				break
			}
		}
		if alike {
			ex = append(ex, fmt.Sprintf("%s: %d × %s", nd.target(), len(nd.kids), first.target()))
			at = append(at, nd.target())
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("structure", "equal-cards", proto.UnslopMedium, "A row of equal cards, each led by an icon",
		"Let what matters most lead: unequal weight, a list, or one feature shown, not a template row.", ex, at...), true
}

func eyebrowPills(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		if r.decls["text-transform"] != "uppercase" {
			continue
		}
		pill := false
		for _, n := range pxValues(r.decls["border-radius"]) {
			pill = pill || n >= 99
		}
		bg := r.decls["background"] + r.decls["background-color"] + r.decls["border"]
		if pill && bg != "" {
			ex = append(ex, r.where())
			at = append(at, r.selectable())
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("structure", "eyebrow-pill", proto.UnslopLow, "Uppercase pills as kickers",
		"Drop the kicker pill; a heading that says it needs no label above it.", ex, at...), true
}

func pureBlackWhite(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		bg := colorsIn(r.decls["background"] + " " + r.decls["background-color"])
		fg := colorsIn(r.decls["color"])
		if len(bg) == 1 && bg[0].a > 0.99 && bg[0].luminance() == 0 && len(fg) == 1 && fg[0].luminance() > 0.99 {
			ex = append(ex, r.where()+": white on pure black")
			at = append(at, r.selectable())
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("color", "pure-black-white", proto.UnslopLow, "Pure white text on pure black",
		"Ease both ends a little, in the design's own hue: glaring contrast tires the eye.", ex, at...), true
}

func shadowSprawl(p page) (proto.UnslopFinding, bool) {
	seen := map[string]bool{}
	for _, r := range p.rules {
		if v := r.decls["box-shadow"]; v != "" && v != "none" {
			seen[v] = true
		}
	}
	if len(seen) < 5 {
		return proto.UnslopFinding{}, false
	}
	var ex []string
	for v := range seen {
		ex = append(ex, short(v))
	}
	slices.Sort(ex)
	f := finding("surfaces", "shadows", proto.UnslopLow, fmt.Sprintf("%d different shadows", len(seen)),
		"Use a few elevation levels, the same everywhere they mean the same.", ex)
	f.Count = len(seen)
	return f, true
}

var chartColorRe = regexp.MustCompile(`#[0-9a-fA-F]{6}\b|rgba?\([^)]*\)`)

// rainbowCharts finds charts in many colors or gradient fills: drawn by a
// script (Chart.js and the like) or as SVG gradients.
func rainbowCharts(p page) (proto.UnslopFinding, bool) {
	var ex []string
	for _, s := range p.scripts {
		if !strings.Contains(s, "datasets") && !strings.Contains(s, "series") {
			continue
		}
		hues := map[int]bool{}
		for _, m := range chartColorRe.FindAllString(s, -1) {
			if c, ok := parseColor(strings.ToLower(m)); ok && c.vivid() {
				h, _, _ := c.hsl()
				hues[int(h)/30] = true
			}
		}
		if len(hues) >= 4 {
			ex = append(ex, fmt.Sprintf("a chart in %d hues", len(hues)))
		}
		if strings.Contains(s, "createLinearGradient") {
			ex = append(ex, "a chart filled with a gradient")
		}
	}
	gradients := 0
	for _, nd := range p.nodes {
		if nd.tag == "lineargradient" || nd.tag == "radialgradient" {
			gradients++
		}
	}
	if gradients >= 2 {
		ex = append(ex, fmt.Sprintf("%d SVG gradients", gradients))
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("data", "rainbow-chart", proto.UnslopMedium, "Charts in many colors or gradients",
		"Color what the chart is about and keep the rest quiet; fills flat; labels and units on it.", ex), true
}

func loops(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		for _, k := range []string{"animation", "animation-iteration-count"} {
			if strings.Contains(r.decls[k], "infinite") {
				ex = append(ex, r.where())
				at = append(at, r.selectable())
				break
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("motion", "loops", proto.UnslopMedium, "Animations that never stop",
		"Stop the loops: motion that repeats forever pulls the eye from the content; keep one for real progress only.", ex, at...), true
}

func focusRemoved(p page) (proto.UnslopFinding, bool) {
	restored := false
	var ex, at []string
	for _, r := range p.rules {
		if strings.Contains(r.selector, ":focus-visible") || strings.Contains(r.selector, ":focus") && (r.decls["outline"] != "none" && r.decls["outline"] != "0" || r.decls["box-shadow"] != "") {
			restored = true
		}
		if (r.decls["outline"] == "none" || r.decls["outline"] == "0") && (strings.Contains(r.selector, "button") || strings.Contains(r.selector, "input") || strings.Contains(r.selector, "a") || strings.Contains(r.selector, ":focus") || r.selector == "*") {
			ex = append(ex, r.where())
			at = append(at, r.selectable())
		}
	}
	if restored || len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("access", "focus-removed", proto.UnslopMedium, "Focus outlines removed",
		"Keep a visible focus: a :focus-visible ring in the design's accent.", ex, at...), true
}

func tinyText(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		var sizes []float64
		if v, ok := r.decls["font-size"]; ok {
			sizes = pxValues(v)
		} else if v, ok := r.decls["font"]; ok {
			if n := pxValues(v); len(n) > 0 {
				sizes = n[:1]
			}
		}
		for _, n := range sizes {
			if n > 0 && n < 11 {
				ex = append(ex, fmt.Sprintf("%s: %gpx", r.where(), n))
				at = append(at, r.selectable())
			}
		}
	}
	if len(ex) < 2 {
		return proto.UnslopFinding{}, false
	}
	return finding("access", "tiny-text", proto.UnslopLow, "Text under 11 px",
		"Keep text readable: 12 px at the least, larger for what people read.", ex, at...), true
}

func emDashes(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		if strings.Contains(tt.s, "—") {
			ex = append(ex, "“"+short(tt.s)+"”")
			at = append(at, tt.target)
		}
	}
	if len(ex) < 3 {
		return proto.UnslopFinding{}, false
	}
	return finding("copy", "em-dashes", proto.UnslopLow, "Em dashes everywhere",
		"Write sentences that don't lean on dashes: a period, a comma or a colon.", ex, at...), true
}

var aiPhrases = regexp.MustCompile(`(?i)\b(let'?s (dive|get started|go|crush)|dive (in|into)|crush it|level up|journey|leverage|harness|streamline|transform your|take .{1,20} to the next level|in today'?s|whether you'?re|look no further|say goodbye to|powered by ai|ai[- ]driven)\b`)

func aiCopy(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		if m := aiPhrases.FindString(tt.s); m != "" {
			ex = append(ex, m+" in “"+short(tt.s)+"”")
			at = append(at, tt.target)
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("copy", "ai-phrases", proto.UnslopMedium, "Stock phrases",
		"Say it in the product's own words, about what it does for this user.", ex, at...), true
}
