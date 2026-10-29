package unslop

import (
	"strings"

	"github.com/enowx/enowx/proto"
)

// A design's theme, dark or light, read from its page's background (or,
// without one, its text): a redesign keeps it.

const (
	Dark  = "dark"
	Light = "light"
)

// pageSelectors are the rules whose background is the page's.
var pageSelectors = []string{"html", "body", ":root", "main", ".app", "#app", ".page", ".wrap", ".container"}

// Theme is html's theme: Dark, Light, or "" when it can't tell.
func Theme(html string) string {
	p := parse(html)
	for _, want := range pageSelectors {
		for _, r := range p.rules {
			if !hasSelector(r.selector, want) {
				continue
			}
			for _, k := range []string{"background-color", "background"} {
				if cs := colorsIn(r.decls[k]); len(cs) > 0 && cs[0].a > 0.9 {
					return themeOf(cs[0], false)
				}
			}
			if cs := colorsIn(r.decls["color"]); len(cs) > 0 {
				return themeOf(cs[0], true)
			}
		}
	}
	return ""
}

func hasSelector(selector, want string) bool {
	for _, s := range strings.Split(selector, ",") {
		if strings.TrimSpace(s) == want {
			return true
		}
	}
	return false
}

// themeOf is the theme of a background c, or of text c (light text: dark).
func themeOf(c rgb, text bool) string {
	dark := c.luminance() < 0.18
	if text {
		dark = c.luminance() > 0.5
	}
	if dark {
		return Dark
	}
	return Light
}

// ThemeFinding is the finding of a redesign that turned theme from to the
// theme of html, ok false when it kept it.
func ThemeFinding(from, html string) (proto.UnslopFinding, bool) {
	now := Theme(html)
	if from == "" || now == "" || now == from {
		return proto.UnslopFinding{}, false
	}
	return finding("color", "theme-changed", proto.UnslopHigh, "The theme changed: it was "+from+", now "+now,
		"Keep the original's theme: a "+from+" design stays "+from+"; unslop changes its care, not its look.", []string{from + " → " + now}, "body"), true
}
