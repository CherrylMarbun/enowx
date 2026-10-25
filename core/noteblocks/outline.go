package noteblocks

import (
	"fmt"
	"strings"
	"unicode"
)

// Section is a part of a note that can be read or written alone: a
// heading and what follows it up to the next heading as high, or a
// titled part of a layout block (a tab, a toggle, a callout, a card).
type Section struct {
	ID    string // unique in the note, from its title: "canvas", "canvas-2"
	Title string
	Kind  string // h1…h6, tab, toggle, callout, card
	Level int    // a heading's level; a part's, its heading's plus its depth
	Depth int    // how many layout blocks it is in
	// Line is its heading or its part's start; its content runs from
	// Line+1 to End, End excluded.
	Line, End int
	// Done and Todos count its to-dos.
	Done, Todos int
}

// Heading says line is a heading, and its level and title.
func Heading(line string) (level int, title string, ok bool) {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return 0, "", false
	}
	for level < len(t) && level < 7 && t[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || len(t) > level && t[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(strings.TrimRight(strings.TrimSpace(t[level:]), "#")), true
}

// Outline is a note's sections, in order.
func Outline(md string) []Section {
	lines := Lines(md)
	var out []Section
	outline(lines, Parse(md), len(lines), 0, 0, &out)
	ids := map[string]int{}
	for i := range out {
		base := slug(out[i].Title)
		if base == "" {
			base = out[i].Kind
		}
		ids[base]++
		out[i].ID = base
		if n := ids[base]; n > 1 {
			out[i].ID = fmt.Sprintf("%s-%d", base, n)
		}
		out[i].Done, out[i].Todos = progress(lines[out[i].Line+1 : out[i].End])
	}
	return out
}

// outline adds the sections of nodes, which end at end, inside depth
// layout blocks under a heading of level base.
func outline(lines []string, nodes []Node, end, depth, base int, out *[]Section) {
	type open struct{ at, level int }
	var heads []open // headings of this scope still running
	closeTo := func(level, at int) {
		for len(heads) > 0 && heads[len(heads)-1].level >= level {
			(*out)[heads[len(heads)-1].at].End = at
			heads = heads[:len(heads)-1]
		}
	}
	for _, n := range nodes {
		if n.Name == "" {
			fenced := false
			for k, line := range n.Lines {
				if isFence(line) {
					fenced = !fenced
				}
				if fenced {
					continue
				}
				if level, title, ok := Heading(line); ok {
					at := n.Start + k
					closeTo(level, at)
					heads = append(heads, open{len(*out), level})
					*out = append(*out, Section{Title: title, Kind: fmt.Sprintf("h%d", level), Level: level, Depth: depth, Line: at, End: end})
				}
			}
			continue
		}
		level := base
		if len(heads) > 0 {
			level = (*out)[heads[len(heads)-1].at].Level
		}
		for _, p := range n.Parts {
			kind := map[string]string{"tabs": "tab", "cards": "card", "toggle": "toggle", "callout": "callout"}[n.Name]
			title := p.Title
			if kind == "toggle" {
				title = n.Args
			}
			if kind == "callout" {
				_, title, _ = strings.Cut(n.Args, " ")
				title = strings.TrimSpace(title)
			}
			if kind != "" && title != "" {
				*out = append(*out, Section{Title: title, Kind: kind, Level: level + 1, Depth: depth + 1, Line: p.Line, End: p.End})
			}
			outline(lines, p.Nodes, p.End, depth+1, level+1, out)
		}
	}
	closeTo(0, end)
}

// slug is an ID made of a title: lower case words joined by "-".
func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
		default:
			dash = true
		}
	}
	return b.String()
}

// Slug is the ID a section titled title has when it's the first so
// titled, as links to it name it.
func Slug(title string) string { return slug(title) }

// Find is the section name names: its ID ("#" before it or not), its
// title, or, when only one has it, a part of its title. The error says
// why there is none: no such section, or several.
func Find(sections []Section, name string) (Section, error) {
	name = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "#"))
	if name == "" {
		return Section{}, fmt.Errorf("no section named")
	}
	for _, s := range sections {
		if s.ID == name {
			return s, nil
		}
	}
	var titled, partly []Section
	low := strings.ToLower(name)
	for _, s := range sections {
		t := strings.ToLower(s.Title)
		switch {
		case t == low || slug(s.Title) == slug(name):
			titled = append(titled, s)
		case strings.Contains(t, low):
			partly = append(partly, s)
		}
	}
	for _, list := range [][]Section{titled, partly} {
		switch len(list) {
		case 0:
			continue
		case 1:
			return list[0], nil
		}
		var ids []string
		for _, s := range list {
			ids = append(ids, "#"+s.ID)
		}
		return Section{}, fmt.Errorf("%q names %d sections: %s", name, len(list), strings.Join(ids, ", "))
	}
	return Section{}, fmt.Errorf("no section %q", name)
}

// Text is section s of md, its heading or part's start included.
func Text(md string, s Section) string {
	lines := Lines(md)
	return strings.Join(lines[s.Line:min(s.End, len(lines))], "\n") + "\n"
}

// Replace is md with the content of section s, under its heading or its
// part's start, made body.
func Replace(md string, s Section, body string) string {
	lines := Lines(md)
	var out []string
	out = append(out, lines[:s.Line+1]...)
	out = append(out, Lines(body)...)
	// A heading's next one keeps a blank line before it.
	if s.End < len(lines) && strings.HasPrefix(s.Kind, "h") && len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		if _, _, ok := Heading(lines[s.End]); ok {
			out = append(out, "")
		}
	}
	out = append(out, lines[min(s.End, len(lines)):]...)
	return strings.Join(out, "\n") + "\n"
}
