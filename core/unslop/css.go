package unslop

import (
	"strings"

	"golang.org/x/net/html"
)

// A design's styles and text, as the rules read them: each CSS rule's
// declarations (a <style> block's, or an element's style attribute), the
// at-rules met (@keyframes…), and the text people read.

// rule is one CSS rule: its selector ("" for a style attribute) and its
// declarations, property → value, lowercased.
type rule struct {
	selector string
	decls    map[string]string
}

type page struct {
	rules    []rule
	atRules  []string // "@keyframes spin", "@media …"
	texts    []string // text nodes, trimmed
	elements int
}

// parse reads a design's HTML.
func parse(src string) page {
	var p page
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return p
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			p.elements++
			switch n.Data {
			case "style":
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						p.readCSS(c.Data)
					}
				}
				return
			case "script", "noscript", "template":
				return
			}
			for _, a := range n.Attr {
				if a.Key == "style" {
					p.rules = append(p.rules, rule{decls: declarations(a.Val)})
				}
			}
		case html.TextNode:
			if t := strings.Join(strings.Fields(n.Data), " "); t != "" {
				p.texts = append(p.texts, t)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return p
}

// readCSS reads a stylesheet's rules, those within @media and the like
// too; @keyframes are only noted.
func (p *page) readCSS(css string) {
	css = stripComments(css)
	for len(css) > 0 {
		open := strings.IndexByte(css, '{')
		if open < 0 {
			return
		}
		head := strings.TrimSpace(css[:open])
		end := matching(css, open)
		body := css[open+1 : end]
		if end < len(css) {
			css = css[end+1:]
		} else {
			css = ""
		}
		switch {
		case strings.HasPrefix(head, "@"):
			p.atRules = append(p.atRules, strings.ToLower(head))
			if !strings.HasPrefix(head, "@keyframes") && !strings.HasPrefix(head, "@font-face") {
				p.readCSS(body)
			}
		default:
			p.rules = append(p.rules, rule{selector: head, decls: declarations(body)})
		}
	}
}

// matching is the index of the brace closing the one at open, or the end.
func matching(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s)
}

func stripComments(css string) string {
	var b strings.Builder
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			b.WriteString(css)
			return b.String()
		}
		b.WriteString(css[:i])
		j := strings.Index(css[i+2:], "*/")
		if j < 0 {
			return b.String()
		}
		css = css[i+2+j+2:]
	}
}

// declarations reads "a:b;c:d", a value's parentheses kept whole.
func declarations(body string) map[string]string {
	out := map[string]string{}
	depth, start := 0, 0
	add := func(s string) {
		k, v, ok := strings.Cut(s, ":")
		if !ok {
			return
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		if k != "" && v != "" {
			out[k] = strings.ToLower(v)
		}
	}
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ';':
			if depth == 0 {
				add(body[start:i])
				start = i + 1
			}
		}
	}
	add(body[start:])
	return out
}
