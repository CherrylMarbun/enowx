package unslop

import (
	"strings"

	"golang.org/x/net/html"
)

// The page's elements as a tree, each with the declarations that apply to
// it: its style attribute's, and those of the rules whose selector's last
// part names it (tag, #id, .class), which is enough to tell a card from a
// box without a browser.

type node struct {
	tag, id string
	classes []string
	parent  int // -1 for none
	kids    []int
	inline  map[string]string
}

func (p *page) addNode(n *html.Node, parent int) int {
	nd := node{tag: n.Data, parent: parent}
	for _, a := range n.Attr {
		switch a.Key {
		case "id":
			nd.id = a.Val
		case "class":
			nd.classes = strings.Fields(a.Val)
		case "style":
			nd.inline = declarations(a.Val)
		}
	}
	p.nodes = append(p.nodes, nd)
	i := len(p.nodes) - 1
	if parent >= 0 {
		p.nodes[parent].kids = append(p.nodes[parent].kids, i)
	}
	return i
}

// target is the node's selector for the page: its #id, else its tag and
// classes.
func (nd node) target() string {
	if nd.id != "" {
		return "#" + cssIdent(nd.id)
	}
	s := nd.tag
	for _, c := range nd.classes {
		s += "." + cssIdent(c)
	}
	return s
}

// matches says compound selector sel (no combinators: "div.card#x")
// names nd.
func (nd node) matches(sel string) bool {
	sel = pseudoRe.ReplaceAllString(strings.TrimSpace(sel), "")
	if sel == "" || strings.ContainsAny(sel, "[*") {
		return false
	}
	tag, rest := sel, ""
	if i := strings.IndexAny(sel, ".#"); i >= 0 {
		tag, rest = sel[:i], sel[i:]
	}
	if tag != "" && tag != nd.tag {
		return false
	}
	for rest != "" {
		kind := rest[0]
		rest = rest[1:]
		j := strings.IndexAny(rest, ".#")
		name := rest
		if j >= 0 {
			name, rest = rest[:j], rest[j:]
		} else {
			rest = ""
		}
		switch kind {
		case '#':
			if name != nd.id {
				return false
			}
		case '.':
			found := false
			for _, c := range nd.classes {
				found = found || c == name
			}
			if !found {
				return false
			}
		}
	}
	return true
}

// style is what applies to node i: matching rules', then its own.
func (p *page) style(i int) map[string]string {
	nd := p.nodes[i]
	out := map[string]string{}
	for _, r := range p.rules {
		if r.selector == "" {
			continue
		}
		for _, part := range strings.Split(r.selector, ",") {
			fields := strings.FieldsFunc(part, func(r rune) bool { return r == ' ' || r == '>' || r == '+' || r == '~' })
			if len(fields) > 0 && nd.matches(fields[len(fields)-1]) {
				for k, v := range r.decls {
					out[k] = v
				}
				break
			}
		}
	}
	for k, v := range nd.inline {
		out[k] = v
	}
	return out
}

// cardLike says declarations d make a card: rounded and set apart, by a
// shadow, or by a border around a surface of its own (a background).
func cardLike(d map[string]string) bool {
	r := pxValues(d["border-radius"])
	if len(r) == 0 || r[0] < 4 {
		return false
	}
	bg := d["background"] + d["background-color"]
	surface := bg != "" && !strings.Contains(bg, "transparent") && !strings.HasPrefix(bg, "none")
	border := d["border"] != "" && d["border"] != "none" && d["border"] != "0"
	shadow := d["box-shadow"] != "" && d["box-shadow"] != "none"
	return shadow || border && surface
}
