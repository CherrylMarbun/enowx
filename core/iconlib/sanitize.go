package iconlib

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// The user's SVG is data from anywhere: it is drawn only once rebuilt from
// shapes and their looks, without what runs or loads (scripts, handlers,
// links out, foreign content).

// Limits on the user's SVG.
const (
	MaxSVG     = 64 << 10 // read
	MaxIconSVG = 24 << 10 // kept as an icon, after cleaning
)

var allowedElems = map[string]bool{
	"svg": true, "g": true, "path": true, "circle": true, "ellipse": true, "line": true,
	"polyline": true, "polygon": true, "rect": true, "defs": true, "lineargradient": true,
	"radialgradient": true, "stop": true, "clippath": true, "mask": true, "use": true,
	"symbol": true, "title": true, "desc": true, "text": true, "tspan": true, "pattern": true,
}

var allowedAttrs = map[string]bool{
	"viewbox": true, "width": true, "height": true, "x": true, "y": true, "x1": true, "y1": true,
	"x2": true, "y2": true, "cx": true, "cy": true, "r": true, "rx": true, "ry": true, "d": true,
	"points": true, "fill": true, "fill-opacity": true, "fill-rule": true, "clip-rule": true,
	"stroke": true, "stroke-width": true, "stroke-linecap": true, "stroke-linejoin": true,
	"stroke-miterlimit": true, "stroke-dasharray": true, "stroke-dashoffset": true,
	"stroke-opacity": true, "opacity": true, "transform": true, "id": true, "offset": true,
	"stop-color": true, "stop-opacity": true, "gradientunits": true, "gradienttransform": true,
	"spreadmethod": true, "fx": true, "fy": true, "clip-path": true, "mask": true, "style": true,
	"preserveaspectratio": true, "font-size": true, "font-family": true, "font-weight": true,
	"text-anchor": true, "dx": true, "dy": true, "patternunits": true, "href": true,
	"vector-effect": true, "xmlns": true,
}

var (
	badURL    = regexp.MustCompile(`(?i)url\(\s*['"]?\s*[^#'"\s)]`)
	badStyle  = regexp.MustCompile(`(?i)expression|javascript:|@import|behavior`)
	colorAttr = map[string]bool{"fill": true, "stroke": true, "stop-color": true}
)

// Sanitize rebuilds svg from what draws, its viewBox set (from its size
// when it has none) and its size the viewBox's; currentColor makes its
// colors the text's.
func Sanitize(svg []byte, currentColor bool) (string, error) {
	if len(svg) > MaxSVG {
		return "", fmt.Errorf("icon: the SVG is %d KB, more than %d", len(svg)>>10, MaxSVG>>10)
	}
	dec := xml.NewDecoder(bytes.NewReader(svg))
	dec.Strict = false
	dec.Entity = map[string]string{} // no entities: no expansion tricks
	var out strings.Builder
	skip := 0 // depth inside an element dropped
	depth := 0
	root := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("icon: the SVG doesn't read: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			if skip > 0 || !allowedElems[name] || (depth == 0 && name != "svg") {
				skip++
				continue
			}
			attrs := cleanAttrs(t.Attr, currentColor)
			if depth == 0 {
				root = true
				attrs = rootAttrs(attrs)
			}
			depth++
			out.WriteString("<" + elemName(name))
			for _, a := range attrs {
				out.WriteString(" " + a[0] + `="` + escape(a[1]) + `"`)
			}
			out.WriteString(">")
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			if depth == 0 {
				continue
			}
			depth--
			out.WriteString("</" + elemName(strings.ToLower(t.Name.Local)) + ">")
		case xml.CharData:
			if skip == 0 && depth > 0 {
				if s := strings.TrimSpace(string(t)); s != "" {
					out.WriteString(escape(s))
				}
			}
		}
	}
	if !root {
		return "", errors.New("icon: that isn't an SVG")
	}
	return out.String(), nil
}

// elemName is an element's name as SVG spells it.
func elemName(lower string) string {
	switch lower {
	case "lineargradient":
		return "linearGradient"
	case "radialgradient":
		return "radialGradient"
	case "clippath":
		return "clipPath"
	}
	return lower
}

var attrNames = map[string]string{
	"viewbox": "viewBox", "gradientunits": "gradientUnits", "gradienttransform": "gradientTransform",
	"spreadmethod": "spreadMethod", "preserveaspectratio": "preserveAspectRatio", "patternunits": "patternUnits",
}

func cleanAttrs(in []xml.Attr, currentColor bool) [][2]string {
	var out [][2]string
	for _, a := range in {
		name := strings.ToLower(a.Name.Local)
		if a.Name.Space == "xmlns" || strings.HasPrefix(name, "on") || !allowedAttrs[name] {
			continue
		}
		v := a.Value
		switch name {
		case "href":
			if !strings.HasPrefix(v, "#") {
				continue // a link out
			}
		case "xmlns":
			v = "http://www.w3.org/2000/svg"
		case "style":
			if badStyle.MatchString(v) || badURL.MatchString(v) {
				continue
			}
			if currentColor {
				v = recolorStyle(v)
			}
		}
		if badURL.MatchString(v) {
			continue
		}
		if currentColor && colorAttr[name] && paints(v) {
			v = "currentColor"
		}
		if n, ok := attrNames[name]; ok {
			name = n
		}
		out = append(out, [2]string{name, v})
	}
	return out
}

// paints says v is a color, not none or a reference.
func paints(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v != "" && v != "none" && v != "transparent" && v != "currentcolor" && !strings.HasPrefix(v, "url(")
}

var styleColor = regexp.MustCompile(`(?i)\b(fill|stroke|stop-color)\s*:\s*([^;]+)`)

func recolorStyle(s string) string {
	return styleColor.ReplaceAllStringFunc(s, func(m string) string {
		sub := styleColor.FindStringSubmatch(m)
		if !paints(sub[2]) {
			return m
		}
		return sub[1] + ":currentColor"
	})
}

// rootAttrs gives the root a viewBox, from its size when it has none,
// and the viewBox's size, so it fits an icon's slot.
func rootAttrs(attrs [][2]string) [][2]string {
	get := func(k string) string {
		for _, a := range attrs {
			if a[0] == k {
				return a[1]
			}
		}
		return ""
	}
	vb := strings.Fields(strings.ReplaceAll(get("viewBox"), ",", " "))
	var w, h float64
	if len(vb) == 4 {
		w, _ = strconv.ParseFloat(vb[2], 64)
		h, _ = strconv.ParseFloat(vb[3], 64)
	}
	if w <= 0 || h <= 0 {
		w, h = length(get("width")), length(get("height"))
		if w <= 0 || h <= 0 {
			w, h = 24, 24
		}
		vb = []string{"0", "0", num(w), num(h)}
	}
	var out [][2]string
	out = append(out, [2]string{"xmlns", "http://www.w3.org/2000/svg"})
	for _, a := range attrs {
		switch a[0] {
		case "xmlns", "viewBox", "width", "height":
			continue
		}
		out = append(out, a)
	}
	return append(out, [2]string{"viewBox", strings.Join(vb, " ")}, [2]string{"width", num(w)}, [2]string{"height", num(h)})
}

func length(s string) float64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "px")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
