package notesio

import (
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTML's tables and inline text as Markdown.

// table writes a table as Markdown, its first row the header.
func (c *htmlConv) table(n *html.Node) string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != html.ElementNode {
				continue
			}
			if ch.DataAtom == atom.Tr {
				var row []string
				for td := ch.FirstChild; td != nil; td = td.NextSibling {
					if td.Type == html.ElementNode && (td.DataAtom == atom.Td || td.DataAtom == atom.Th) {
						cell := strings.ReplaceAll(collapse(c.text(td)), "|", `\|`)
						row = append(row, strings.ReplaceAll(cell, "\n", " "))
					}
				}
				rows = append(rows, row)
				continue
			}
			walk(ch)
		}
	}
	walk(n)
	return markdownTable(rows)
}

// markdownTable writes rows as a Markdown table, the first the header.
func markdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	var b strings.Builder
	for i, r := range rows {
		cells := make([]string, cols)
		copy(cells, r)
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		if i == 0 {
			b.WriteString("|" + strings.Repeat(" --- |", cols) + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// text is n's inline Markdown.
func (c *htmlConv) text(n *html.Node) string {
	var b strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if isBlock(ch) && ch.DataAtom != atom.Br {
			b.WriteString("\n")
			b.WriteString(c.text(ch))
			b.WriteString("\n")
			continue
		}
		c.inline(&b, ch)
	}
	return strings.TrimSpace(collapse(b.String()))
}

// inline writes n as inline Markdown.
func (c *htmlConv) inline(b *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(spaces.ReplaceAllString(n.Data, " "))
		return
	case html.ElementNode:
	default:
		return
	}
	if dropped[n.DataAtom] {
		return
	}
	if what := unconvertible[n.DataAtom]; what != "" {
		c.batch.unconverted(c.where, what)
		return
	}
	// A mark goes around the words, the spaces around it outside.
	wrap := func(mark string) {
		var in strings.Builder
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			c.inline(&in, ch)
		}
		raw := in.String()
		inner := strings.TrimSpace(raw)
		if inner == "" {
			b.WriteString(raw)
			return
		}
		if raw[0] == ' ' {
			b.WriteString(" ")
		}
		b.WriteString(mark + inner + mark)
		if raw[len(raw)-1] == ' ' {
			b.WriteString(" ")
		}
	}
	switch n.DataAtom {
	case atom.Strong, atom.B:
		wrap("**")
	case atom.Em, atom.I:
		wrap("*")
	case atom.S, atom.Del, atom.Strike:
		wrap("~~")
	case atom.Code:
		if t := rawText(n); t != "" {
			b.WriteString("`" + t + "`")
		}
	case atom.Br:
		b.WriteString("\n")
	case atom.A:
		label := strings.TrimSpace(c.text(n))
		href := attr(n, "href")
		if c.link != nil {
			if md, ok := c.link(href); ok {
				if md == "" {
					md = label
				}
				b.WriteString(md)
				return
			}
		}
		if u := safeURL(href); u != "" && label != "" {
			b.WriteString("[" + label + "](" + u + ")")
		} else {
			b.WriteString(label)
		}
	case atom.Img:
		if md := c.img(attr(n, "src"), attr(n, "alt")); md != "" {
			b.WriteString(md)
		}
	default:
		if n.Data == "en-media" && c.media != nil {
			b.WriteString(c.media(attr(n, "hash"), attr(n, "type")))
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			c.inline(b, ch)
		}
	}
}

func (c *htmlConv) img(src, alt string) string {
	if c.image != nil {
		return c.image(src, alt)
	}
	if u := safeURL(src); u != "" {
		return "![" + alt + "](" + u + ")"
	}
	return ""
}

// safeURL is u when it is a web or mail address, "" otherwise.
func safeURL(u string) string {
	p, err := url.Parse(strings.TrimSpace(u))
	if err != nil {
		return ""
	}
	switch strings.ToLower(p.Scheme) {
	case "http", "https", "mailto":
		return strings.ReplaceAll(p.String(), " ", "%20")
	}
	return ""
}

// rawText is the text under n as it is, spaces kept.
func rawText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && ch.DataAtom == atom.Br {
			b.WriteString("\n")
			continue
		}
		b.WriteString(rawText(ch))
	}
	return b.String()
}

// collapse joins a line's runs of spaces, keeping its line breaks.
func collapse(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// tidy trims a body: no blank lines at its ends, none more than one.
func tidy(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimRight(l, " \t")
		if l == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, l)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}
