package unslop

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The checks of truth (invented data), side stripes and generic buttons.

var placeholderRe = regexp.MustCompile(`(?i)\b(john doe|jane doe|jane smith|john smith|sarah johnson|sarah chen|michael chen|acme( inc| corp)?|globex|initech|example (inc|corp|co)|lorem ipsum|dolor sit amet|(card|feature|section|item) (title|description)|(title|description) (goes )?here)\b`)

// magicRe are round, too-good numbers: 10x, 99.9%, 10,000+, 4.9★.
var magicRe = regexp.MustCompile(`(?i)\b(10x|99\.9+%|\d{1,3}(,000)+\+|\d+k\+|4\.9\s*★)`)

func fakeData(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		t := tt.s
		for _, m := range placeholderRe.FindAllString(t, -1) {
			ex = append(ex, "“"+m+"”")
			at = append(at, tt.target)
		}
		for _, m := range magicRe.FindAllString(t, -1) {
			ex = append(ex, "“"+m+"”")
			at = append(at, tt.target)
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("truth", "fake-data", proto.UnslopHigh, "Invented names, brands or numbers",
		"Use the product's real data, or a labelled placeholder ([REAL DATA], “Your name”), or remove the section.", ex, at...), true
}

var deltaRe = regexp.MustCompile(`[+−-]\s?\d+(\.\d+)?\s?%`)

// periodRe names what a change compares with.
var periodRe = regexp.MustCompile(`(?i)\b(vs|versus|since|from|than|last|this|per|today|yesterday|week|month|year|quarter|day|q[1-4]|ytd|mom|yoy)\b`)

func unlabelledDeltas(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, tt := range p.texts {
		t := tt.s
		if d := deltaRe.FindString(t); d != "" && !periodRe.MatchString(t) {
			ex = append(ex, "“"+short(t)+"”")
			at = append(at, tt.target)
		}
	}
	if len(ex) < 2 {
		return proto.UnslopFinding{}, false
	}
	return finding("truth", "delta-period", proto.UnslopMedium, "Changes without a period",
		"Name what each change compares with: “+12% vs last month”.", ex, at...), true
}

func sideStripes(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	for _, r := range p.rules {
		for _, k := range []string{"border-left", "border-top", "border-inline-start"} {
			v, ok := r.decls[k]
			if !ok {
				continue
			}
			wide := false
			for _, n := range pxValues(v) {
				wide = wide || n >= 3
			}
			if wide && len(colorsIn(v)) > 0 && colorsIn(v)[0].vivid() && r.decls["border-radius"] != "" {
				ex = append(ex, r.where()+" "+k)
				at = append(at, r.selectable())
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	return finding("surfaces", "side-stripe", proto.UnslopMedium, "Colored stripes on rounded cards",
		"Remove the stripe, or make it a real state (an error, a selection) shown with a label too.", ex, at...), true
}

var genericCTAs = []string{"get started", "learn more", "read more", "try now", "explore", "discover", "submit", "click here", "start now", "sign up now", "join now"}

// genericButtons finds generic labels; one said three times or more (a
// "Learn more" on every card) weighs more: the template's repetition.
func genericButtons(p page) (proto.UnslopFinding, bool) {
	var ex, at []string
	times := map[string]int{}
	for _, tt := range p.texts {
		t := tt.s
		low := strings.ToLower(strings.TrimRight(strings.TrimSpace(t), "→›» !"))
		for _, c := range genericCTAs {
			if low == c {
				ex = append(ex, strconv.Quote(t))
				at = append(at, tt.target)
				times[c]++
			}
		}
	}
	if len(ex) == 0 {
		return proto.UnslopFinding{}, false
	}
	f := finding("copy", "generic-cta", proto.UnslopLow, "Generic button labels",
		"Say the action and its object: “Create project”, “Import notes”, “Start a run”; a link names where it goes.", ex, at...)
	for _, c := range genericCTAs {
		if n := times[c]; n >= 3 {
			f.Severity, f.Title = proto.UnslopMedium, "The same generic button again and again"
			f.Examples = append([]string{fmt.Sprintf("“%s” %d times", c, n)}, f.Examples...)
		}
	}
	return f, true
}
