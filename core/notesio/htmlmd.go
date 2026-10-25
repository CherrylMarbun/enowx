package notesio

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// HTML becomes Markdown here, and nothing of it runs: scripts, styles,
// forms and frames are left out, links keep web and mail addresses only,
// and no HTML is written out.

var spaces = regexp.MustCompile(`\s+`)

// htmlConv turns HTML into Markdown for one note.
type htmlConv struct {
	batch *Batch
	where string
	// link rewrites a link's address: a note's becomes [[key]] (ok false
	// keeps the label alone); nil keeps web and mail links.
	link func(href string) (md string, ok bool)
	// image is the Markdown of an image, "" to leave it out; nil keeps web
	// images.
	image func(src, alt string) string
	// media is the Markdown of an Evernote <en-media hash=…>.
	media func(hash, kind string) string

	out   strings.Builder
	lists []rune // the kinds of the lists we are in: '-' or '1'
}

// convert turns the HTML document or fragment src.
func (c *htmlConv) convert(src string) string {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return ""
	}
	body := find(doc, atom.Body)
	if body == nil {
		body = doc
	}
	c.blocks(body, "")
	return tidy(c.out.String())
}

// find is the first element a under n.
func find(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if f := find(ch, a); f != nil {
			return f
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

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

// dropped are the elements nothing of goes into a note.
var dropped = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Head: true, atom.Noscript: true, atom.Template: true,
	atom.Form: true, atom.Button: true, atom.Select: true, atom.Textarea: true, atom.Link: true, atom.Meta: true,
	atom.Title: true, atom.Svg: true, atom.Canvas: true, atom.Object: true,
}

// unconvertible are the elements only a link of is kept, if that.
var unconvertible = map[atom.Atom]string{
	atom.Iframe: "an embedded page", atom.Embed: "an embed", atom.Video: "a video", atom.Audio: "an audio clip", atom.Math: "an equation",
}

// isBlock says n starts a block of its own.
func isBlock(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.DataAtom {
	case atom.P, atom.Div, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Ul, atom.Ol, atom.Li,
		atom.Pre, atom.Blockquote, atom.Table, atom.Hr, atom.Figure, atom.Aside, atom.Details, atom.Section,
		atom.Article, atom.Header, atom.Footer, atom.Main, atom.Nav, atom.Dl, atom.Dt, atom.Dd, atom.Br:
		return true
	}
	return n.Data == "en-note"
}

// para writes a block of text: a blank line before it, unless it starts
// the note or follows a list item or quote line of the same kind.
func (c *htmlConv) para(prefix, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if c.out.Len() > 0 && !strings.HasSuffix(c.out.String(), "\n\n") {
		c.out.WriteString("\n")
	}
	if prefix == "" {
		c.out.WriteString(text + "\n\n")
		return
	}
	for _, line := range strings.Split(text, "\n") {
		c.out.WriteString(strings.TrimRight(prefix+line, " ") + "\n")
	}
	c.out.WriteString("\n")
}

// line writes one line of a list, with no blank line around it.
func (c *htmlConv) line(s string) {
	if c.out.Len() > 0 && !strings.HasSuffix(c.out.String(), "\n") {
		c.out.WriteString("\n")
	}
	c.out.WriteString(s + "\n")
}

// blocks writes n's children as blocks, each line after prefix (a
// quote's "> ").
func (c *htmlConv) blocks(n *html.Node, prefix string) {
	var inline []*html.Node
	flush := func() {
		if len(inline) == 0 {
			return
		}
		var b strings.Builder
		for _, in := range inline {
			c.inline(&b, in)
		}
		inline = nil
		c.para(prefix, b.String())
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if !isBlock(ch) && !(ch.Type == html.ElementNode && (dropped[ch.DataAtom] || unconvertible[ch.DataAtom] != "")) {
			inline = append(inline, ch)
			continue
		}
		flush()
		c.block(ch, prefix)
	}
	flush()
}

// block writes element n, a block.
func (c *htmlConv) block(n *html.Node, prefix string) {
	if dropped[n.DataAtom] {
		return
	}
	if what := unconvertible[n.DataAtom]; what != "" {
		c.batch.unconverted(c.where, what)
		if src := safeURL(attr(n, "src")); src != "" {
			c.para(prefix, "["+what+"]("+src+")")
		}
		return
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := min(int(n.Data[1]-'0'), 3)
		c.para(prefix, strings.Repeat("#", level)+" "+c.text(n))
	case atom.Ul, atom.Ol:
		kind := '-'
		if n.DataAtom == atom.Ol {
			kind = '1'
		}
		c.lists = append(c.lists, kind)
		for li := n.FirstChild; li != nil; li = li.NextSibling {
			if li.Type == html.ElementNode && li.DataAtom == atom.Li {
				c.item(li, prefix, kind)
			}
		}
		c.lists = c.lists[:len(c.lists)-1]
		if len(c.lists) == 0 {
			c.line(strings.TrimRight(prefix, " "))
		}
	case atom.Li:
		c.item(n, prefix, '-')
	case atom.Pre:
		code := n
		if f := find(n, atom.Code); f != nil {
			code = f
		}
		lang := ""
		for _, cl := range strings.Fields(attr(code, "class")) {
			if l, ok := strings.CutPrefix(cl, "language-"); ok {
				lang = l
			}
		}
		text := strings.TrimRight(rawText(code), "\n")
		c.para(prefix, "```"+lang+"\n"+text+"\n```")
	case atom.Blockquote:
		c.blocks(n, prefix+"> ")
	case atom.Figure, atom.Aside:
		if n.DataAtom == atom.Aside || hasClass(n, "callout") {
			// A callout: its icon, then what it says, as a quote.
			c.para(prefix, "> "+strings.ReplaceAll(strings.TrimSpace(c.text(n)), "\n", "\n> "))
			return
		}
		c.blocks(n, prefix)
	case atom.Details:
		if s := find(n, atom.Summary); s != nil {
			c.para(prefix, "**"+strings.TrimSpace(c.text(s))+"**")
			s.Parent.RemoveChild(s)
		}
		c.blocks(n, prefix)
	case atom.Table:
		c.para(prefix, c.table(n))
	case atom.Hr:
		c.para(prefix, "---")
	case atom.Br:
	default:
		if n.Data == "en-todo" {
			return
		}
		if c.todo(n) {
			return
		}
		c.blocks(n, prefix)
	}
}

// todo writes n as a to-do when it holds a checkbox before its text (an
// Evernote <en-todo>, a checkbox input), and says whether it did.
func (c *htmlConv) todo(n *html.Node) bool {
	box := firstBox(n)
	if box == nil {
		return false
	}
	checked := attr(box, "checked") != "" && attr(box, "checked") != "false"
	unbox(box)
	mark := " "
	if checked {
		mark = "x"
	}
	c.line(fmt.Sprintf("%s- [%s] %s", strings.Repeat("  ", len(c.lists)), mark, strings.TrimSpace(c.text(n))))
	return true
}

// unbox takes a checkbox out, keeping what HTML's parser put in it (an
// <en-todo/> isn't empty to it).
func unbox(box *html.Node) {
	for ch := box.FirstChild; ch != nil; {
		next := ch.NextSibling
		box.RemoveChild(ch)
		box.Parent.InsertBefore(ch, box)
		ch = next
	}
	box.Parent.RemoveChild(box)
}

// firstBox is the checkbox n starts with, if it does.
func firstBox(n *html.Node) *html.Node {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.TextNode && strings.TrimSpace(ch.Data) == "" {
			continue
		}
		if ch.Type != html.ElementNode {
			return nil
		}
		if ch.Data == "en-todo" || ch.DataAtom == atom.Input && attr(ch, "type") == "checkbox" {
			return ch
		}
		return firstBox(ch)
	}
	return nil
}

// item writes a list item, and the lists in it under it.
func (c *htmlConv) item(li *html.Node, prefix string, kind rune) {
	depth := max(len(c.lists)-1, 0)
	indent := strings.Repeat("  ", depth)
	mark := "- "
	if kind == '1' {
		mark = "1. "
	}
	// Notion's to-dos: a checkbox div, checked by its class.
	if box := firstBox(li); box != nil {
		checked := attr(box, "checked") != "" && attr(box, "checked") != "false"
		unbox(box)
		if checked {
			mark = "- [x] "
		} else {
			mark = "- [ ] "
		}
	} else if b := findClass(li, "checkbox"); b != nil {
		if hasClass(b, "checkbox-on") {
			mark = "- [x] "
		} else {
			mark = "- [ ] "
		}
	}
	var text strings.Builder
	var nested []*html.Node
	for ch := li.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && (ch.DataAtom == atom.Ul || ch.DataAtom == atom.Ol) {
			nested = append(nested, ch)
			continue
		}
		if ch.Type == html.ElementNode && isBlock(ch) && ch.DataAtom != atom.Br {
			text.WriteString(" " + c.text(ch) + " ")
			continue
		}
		c.inline(&text, ch)
	}
	c.line(prefix + indent + mark + collapse(text.String()))
	for _, l := range nested {
		c.block(l, prefix)
	}
}

func findClass(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode && hasClass(n, class) {
		return n
	}
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if f := findClass(ch, class); f != nil {
			return f
		}
	}
	return nil
}
