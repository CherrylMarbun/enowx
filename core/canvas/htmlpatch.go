package canvas

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// A mockup's HTML is changed an element at a time, by its id attribute,
// so an agent writes a page piece by piece and the user sees each piece
// as it comes, and a change to one element leaves the others as they are.

// doc is a frame's HTML parsed, and whether it was a whole document
// (written back whole) or a fragment (written back as head and body's
// children).
type doc struct {
	root  *html.Node
	whole bool
}

func parseDoc(src string) (doc, error) {
	root, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return doc{}, fmt.Errorf("canvas: html: %w", err)
	}
	l := strings.ToLower(src)
	return doc{root: root, whole: strings.Contains(l, "<html") || strings.Contains(l, "<!doctype")}, nil
}

func (d doc) render() string {
	var b bytes.Buffer
	if d.whole {
		_ = html.Render(&b, d.root)
		return b.String()
	}
	for _, part := range []atom.Atom{atom.Head, atom.Body} {
		n := find(d.root, func(n *html.Node) bool { return n.Type == html.ElementNode && n.DataAtom == part })
		if n == nil {
			continue
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			_ = html.Render(&b, ch)
		}
	}
	return b.String()
}

func find(n *html.Node, match func(*html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if got := find(ch, match); got != nil {
			return got
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func (d doc) byID(id string) *html.Node {
	return find(d.root, func(n *html.Node) bool { return n.Type == html.ElementNode && attr(n, "id") == id })
}

func (d doc) body() *html.Node {
	return find(d.root, func(n *html.Node) bool { return n.Type == html.ElementNode && n.DataAtom == atom.Body })
}

// fragment parses src as the children of parent.
func fragment(src string, parent *html.Node) ([]*html.Node, error) {
	nodes, err := html.ParseFragment(strings.NewReader(src), parent)
	if err != nil {
		return nil, fmt.Errorf("canvas: html: %w", err)
	}
	return nodes, nil
}

// errNoElement is a patch of an element the frame doesn't have.
var errNoElement = errors.New("no element")

// patchHTML puts src in place of the element with id target, or at the
// end of element into ("" for the body) when target is "" or no element
// has it (a new element, its id in src).
func patchHTML(page, target, into, src string) (string, error) {
	d, err := parseDoc(page)
	if err != nil {
		return "", err
	}
	if n := d.byID(target); target != "" && n != nil {
		nodes, err := fragment(src, n.Parent)
		if err != nil {
			return "", err
		}
		for _, nn := range nodes {
			n.Parent.InsertBefore(nn, n)
		}
		n.Parent.RemoveChild(n)
		return d.render(), nil
	}
	parent := d.body()
	if into != "" {
		if parent = d.byID(into); parent == nil {
			return "", fmt.Errorf("canvas: %w with id %q to put it in", errNoElement, into)
		}
	}
	nodes, err := fragment(src, parent)
	if err != nil {
		return "", err
	}
	for _, nn := range nodes {
		parent.AppendChild(nn)
	}
	return d.render(), nil
}

// removeHTML removes the element with id target.
func removeHTML(page, target string) (string, error) {
	d, err := parseDoc(page)
	if err != nil {
		return "", err
	}
	n := d.byID(target)
	if n == nil {
		return "", fmt.Errorf("canvas: %w with id %q", errNoElement, target)
	}
	n.Parent.RemoveChild(n)
	return d.render(), nil
}

// innerHTML puts src inside the element with id target, in place of what
// it held.
func innerHTML(page, target, src string) (string, error) {
	d, err := parseDoc(page)
	if err != nil {
		return "", err
	}
	n := d.byID(target)
	if n == nil {
		return "", fmt.Errorf("canvas: %w with id %q", errNoElement, target)
	}
	nodes, err := fragment(src, n)
	if err != nil {
		return "", err
	}
	for n.FirstChild != nil {
		n.RemoveChild(n.FirstChild)
	}
	for _, nn := range nodes {
		n.AppendChild(nn)
	}
	return d.render(), nil
}

// HTMLElement is an element of a mockup with an id: what it is, how deep,
// and the start of its text.
type HTMLElement struct {
	ID    string
	Tag   string
	Depth int
	Text  string
}

// Outline is the elements of page with an id, in the page's order:
// what agents and the user change one at a time.
func Outline(page string) []HTMLElement {
	if strings.TrimSpace(page) == "" {
		return nil
	}
	d, err := parseDoc(page)
	if err != nil {
		return nil
	}
	var out []HTMLElement
	var walk func(n *html.Node, depth int)
	walk = func(n *html.Node, depth int) {
		if n.Type == html.ElementNode {
			if id := attr(n, "id"); id != "" {
				out = append(out, HTMLElement{ID: id, Tag: n.Data, Depth: depth, Text: textOf(n, 60)})
				depth++
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch, depth)
		}
	}
	walk(d.root, 0)
	return out
}

// textOf is n's text, its spaces folded, cut at max runes.
func textOf(n *html.Node, max int) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		if n.Type == html.ElementNode && (n.DataAtom == atom.Style || n.DataAtom == atom.Script) {
			return
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	s := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max-1]) + "…"
	}
	return s
}

// ElementHTML is the outer HTML of page's element with id target.
func ElementHTML(page, target string) (string, error) {
	d, err := parseDoc(page)
	if err != nil {
		return "", err
	}
	n := d.byID(target)
	if n == nil {
		return "", fmt.Errorf("canvas: %w with id %q", errNoElement, target)
	}
	var b bytes.Buffer
	_ = html.Render(&b, n)
	return b.String(), nil
}

// ParentID is the id of the element holding page's element target, ""
// for the body (or one without an id).
func ParentID(page, target string) string {
	d, err := parseDoc(page)
	if err != nil {
		return ""
	}
	n := d.byID(target)
	if n == nil || n.Parent == nil || n.Parent.Type != html.ElementNode {
		return ""
	}
	return attr(n.Parent, "id")
}

// AssetNames are the canvas's files page shows (assets/NAME).
func AssetNames(page string) []string {
	var out []string
	for rest := page; ; {
		i := strings.Index(rest, "assets/")
		if i < 0 {
			return out
		}
		rest = rest[i+len("assets/"):]
		end := strings.IndexAny(rest, "\"' )>")
		if end < 0 {
			end = len(rest)
		}
		if name := rest[:end]; name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
}
