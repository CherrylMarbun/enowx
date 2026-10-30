// Package designbrief reads and writes a design's brief
// (proto.DesignBrief) as Markdown, a section a field, so agents and people
// write it the same way.
package designbrief

import (
	"errors"
	"strings"

	"github.com/enowx/enowx/proto"
)

// A field of the brief: its heading and what it asks.
type field struct {
	heading, asks string
	required      bool
	of            func(*proto.DesignBrief) *string
}

var fields = []field{
	{"Product", "What it is, and its category.", true, func(b *proto.DesignBrief) *string { return &b.Product }},
	{"Users", "Who uses it, and the task they come for.", true, func(b *proto.DesignBrief) *string { return &b.Users }},
	{"Tone", "Its look as decisions, not adjectives: calm or lively, dense or airy, polished or raw; then the type, the space, and how much of a screen carries color.", true, func(b *proto.DesignBrief) *string { return &b.Tone }},
	{"First screen", "What the first screen is for (the work itself, or a designed opening), its one focal point and its one main action.", true, func(b *proto.DesignBrief) *string { return &b.FirstScreen }},
	{"Peers", "Two or three products of its kind with the most distinctive looks: what to keep of them, what to do differently.", false, func(b *proto.DesignBrief) *string { return &b.Peers }},
	{"Remove", "What to take away: what repeats, explains the obvious, or decorates without helping.", false, func(b *proto.DesignBrief) *string { return &b.Remove }},
	{"Fixed", "What stays: brand, devices, assets, the project's design system, technical limits.", false, func(b *proto.DesignBrief) *string { return &b.Fixed }},
}

// Path is where a project keeps its brief, from its folder.
const Path = ".enowx/brief.md"

// Template is an empty brief to fill in, each section saying what it asks.
func Template() string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString("## " + f.heading + "\n" + f.asks)
		if !f.required {
			b.WriteString(" (optional)")
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// Markdown is brief b as Markdown, its empty sections left out.
func Markdown(b proto.DesignBrief) string {
	var out strings.Builder
	out.WriteString("# Design brief\n\n")
	for _, f := range fields {
		if v := strings.TrimSpace(*f.of(&b)); v != "" {
			out.WriteString("## " + f.heading + "\n" + v + "\n\n")
		}
	}
	return out.String()
}

// Parse reads a brief from Markdown, a "## Heading" a field; text under
// other headings is left out.
func Parse(md string) proto.DesignBrief {
	var b proto.DesignBrief
	var cur *string
	var text []string
	flush := func() {
		if cur != nil {
			*cur = strings.TrimSpace(strings.Join(text, "\n"))
		}
		cur, text = nil, nil
	}
	for line := range strings.SplitSeq(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		if h, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			if f, ok := named(h); ok {
				cur = f.of(&b)
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			flush()
			continue
		}
		if cur != nil {
			text = append(text, line)
		}
	}
	flush()
	return b
}

func named(heading string) (field, bool) {
	h := strings.ToLower(strings.TrimSpace(strings.TrimRight(heading, ":")))
	for _, f := range fields {
		if strings.ToLower(f.heading) == h {
			return f, true
		}
	}
	return field{}, false
}

// Check says what brief b lacks of what is required, nil when nothing.
func Check(b proto.DesignBrief) error {
	var missing []string
	for _, f := range fields {
		if f.required && strings.TrimSpace(*f.of(&b)) == "" {
			missing = append(missing, f.heading)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return errors.New("the brief has no " + strings.Join(missing, ", ") + ": a section each (## " + strings.Join(missing, ", ## ") + ")")
}

// Summary is brief b in a line: its product and tone.
func Summary(b proto.DesignBrief) string {
	first := func(s string) string {
		s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
		return s
	}
	return strings.Trim(first(b.Product)+" · "+first(b.Tone), " ·")
}

// Fields are a brief's sections as headings and values, in order, for a
// view or a form; Set changes one by its heading.
func Fields(b proto.DesignBrief) [][2]string {
	out := make([][2]string, len(fields))
	for i, f := range fields {
		out[i] = [2]string{f.heading, *f.of(&b)}
	}
	return out
}

// Set changes brief b's section heading to v.
func Set(b *proto.DesignBrief, heading, v string) {
	if f, ok := named(heading); ok {
		*f.of(b) = v
	}
}

// Asks is what section heading asks for.
func Asks(heading string) string {
	f, _ := named(heading)
	return f.asks
}
