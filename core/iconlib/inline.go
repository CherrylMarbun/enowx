package iconlib

import (
	"encoding/base64"
	"regexp"
	"strings"
)

// In a note's text, :icon[lucide:check] is an icon in the line, and a
// fenced block of language svg a drawing of the user's.

// InlinePattern finds the icons in a line; its first group is the icon.
var InlinePattern = regexp.MustCompile(`:icon\[([^\]\s]+)\]`)

// Inline is the icon :icon[ref] writes.
func Inline(ref string) string { return ":icon[" + ref + "]" }

// ExportMarkdown is md readable outside enowX: its icons and drawings
// images of themselves, inline SVG as data, which Markdown viewers show.
func ExportMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "```svg" {
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) != "```" {
				j++
			}
			if j < len(lines) {
				if clean, err := Sanitize([]byte(strings.Join(lines[i+1:j], "\n")), false); err == nil {
					out = append(out, "![drawing]("+dataURI(clean)+")")
					i = j
					continue
				}
			}
		}
		out = append(out, InlinePattern.ReplaceAllStringFunc(lines[i], func(m string) string {
			r, err := Parse(InlinePattern.FindStringSubmatch(m)[1])
			if err != nil {
				return m
			}
			svg := r.SVG()
			if r.Color != "" {
				svg = strings.ReplaceAll(svg, "currentColor", r.Color)
			}
			alt := r.Name
			if alt == "" {
				alt = "icon"
			}
			return `<img alt="` + alt + `" src="` + dataURI(svg) + `" width="16" height="16">`
		}))
	}
	return strings.Join(out, "\n")
}

func dataURI(svg string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}
