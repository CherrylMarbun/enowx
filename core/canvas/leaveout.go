package canvas

import (
	"bytes"
	"slices"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// LeaveOut is page current without what one step of its making changed,
// from before to after: each element (by its id) and style block the step
// changed, and nothing changed since, put back as it was before; an
// element the step added, and nothing changed since, taken out. back are
// the elements put back or taken out, kept those the step changed that
// changed again since, which stay as they are.
func LeaveOut(current, before, after string) (page string, back, kept []string, err error) {
	cur, err := parseDoc(current)
	if err != nil {
		return "", nil, nil, err
	}
	bef, err := parseDoc(before)
	if err != nil {
		return "", nil, nil, err
	}
	aft, err := parseDoc(after)
	if err != nil {
		return "", nil, nil, err
	}
	done := map[string]bool{} // put back whole, those in it with it
	inDone := func(id string) bool {
		for p := ParentID(after, id); p != ""; p = ParentID(after, p) {
			if done[p] {
				return true
			}
		}
		return false
	}
	for _, el := range Outline(after) {
		id := el.ID
		if inDone(id) {
			continue
		}
		a := outer(aft.byID(id))
		b := outer(bef.byID(id))
		if a == b {
			continue
		}
		n := cur.byID(id)
		if n == nil || outer(n) != a {
			kept = append(kept, id)
			continue
		}
		if b == "" { // added by the step
			n.Parent.RemoveChild(n)
		} else {
			nodes, err := fragment(b, n.Parent)
			if err != nil {
				return "", nil, nil, err
			}
			for _, nn := range nodes {
				n.Parent.InsertBefore(nn, n)
			}
			n.Parent.RemoveChild(n)
		}
		done[id] = true
		back = append(back, id)
	}
	// Style blocks, by their place among the page's.
	cs, bs, as := styles(cur), styles(bef), styles(aft)
	if len(cs) == len(as) && len(bs) == len(as) {
		for i := range as {
			t, bt := text(as[i]), text(bs[i])
			if t == bt {
				continue
			}
			if text(cs[i]) != t {
				kept = append(kept, "style block "+strconv.Itoa(i+1))
				continue
			}
			for ch := cs[i].FirstChild; ch != nil; ch = cs[i].FirstChild {
				cs[i].RemoveChild(ch)
			}
			cs[i].AppendChild(&html.Node{Type: html.TextNode, Data: bt})
			back = append(back, "style block "+strconv.Itoa(i+1))
		}
	}
	return cur.render(), back, slices.Compact(kept), nil
}

func outer(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b bytes.Buffer
	_ = html.Render(&b, n)
	return b.String()
}

func styles(d doc) []*html.Node {
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Style && attr(n, "id") == "" {
			out = append(out, n)
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(d.root)
	return out
}

func text(n *html.Node) string {
	var b bytes.Buffer
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.TextNode {
			b.WriteString(ch.Data)
		}
	}
	return b.String()
}
