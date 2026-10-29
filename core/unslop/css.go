package unslop

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// A design's styles and text, as the rules read them: each CSS rule's
// declarations (a <style> block's, or an element's style attribute), the
// at-rules met (@keyframes…), and the text people read.

// rule is one CSS rule: its selector ("" for a style attribute, then
// target its element's "#id" when it has one) and its declarations,
// property → value, lowercased.
type rule struct {
	selector string
	target   string
	decls    map[string]string
}

// text is a text node, trimmed, and the "#id" of its nearest element that
// has one.
type text struct {
	s, target string
}

type page struct {
	rules    []rule
	atRules  []string // "@keyframes spin", "@media …"
	texts    []text
	elements int
	nodes    []node   // the elements, parents before children (tree.go)
	scripts  []string // scripts' text, as charts are drawn
}

// parse reads a design's HTML.
func parse(src string) page {
	var p page
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return p
	}
	var walk func(n *html.Node, near string, parent int)
	walk = func(n *html.Node, near string, parent int) {
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
			case "script":
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.TextNode {
						p.scripts = append(p.scripts, c.Data)
					}
				}
				return
			case "noscript", "template":
				return
			}
			parent = p.addNode(n, parent)
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val != "" {
					near = "#" + cssIdent(a.Val)
				}
			}
			for _, a := range n.Attr {
				if a.Key == "style" {
					p.rules = append(p.rules, rule{target: near, decls: declarations(a.Val)})
				}
			}
		case html.TextNode:
			if t := strings.Join(strings.Fields(n.Data), " "); t != "" {
				p.texts = append(p.texts, text{s: t, target: near})
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, near, parent)
		}
	}
	walk(doc, "", -1)
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

// cssIdent escapes an id for a selector: what isn't a letter, digit, - or
// _ is escaped.
func cssIdent(id string) string {
	var b strings.Builder
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_', r == '-', r >= 0x80, r >= '0' && r <= '9' && i > 0:
			b.WriteRune(r)
		default:
			b.WriteString(`\` + string(r))
		}
	}
	return b.String()
}

// selectable is a rule's selector as the page can query it: pseudo
// elements and states taken out (::after, :hover), "" for none left.
func (r rule) selectable() string {
	if r.selector == "" {
		return r.target
	}
	var out []string
	for _, part := range strings.Split(r.selector, ",") {
		part = pseudoRe.ReplaceAllString(strings.TrimSpace(part), "")
		if part != "" && part != "*" && part != "html" && part != "body" && !strings.HasPrefix(part, ":") {
			out = append(out, part)
		}
	}
	return strings.Join(out, ", ")
}

var pseudoRe = regexp.MustCompile(`::?[a-z-]+(\([^)]*\))?`)
