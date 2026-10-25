package notesio

import (
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// Notes as one HTML page: each a section with its title, a contents list
// first when there are several, their images in the page itself.

const pageStyle = `body{font:15px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",Inter,sans-serif;max-width:760px;margin:40px auto;padding:0 20px;color:#1f2328}
h1{font-size:28px;margin:48px 0 8px}h2{font-size:21px}h3{font-size:17px}.meta{color:#6e7781;font-size:12px}
pre{background:#f6f8fa;padding:12px;border-radius:8px;overflow:auto}code{background:#f6f8fa;padding:1px 4px;border-radius:4px}
blockquote{border-left:3px solid #d0d7de;margin:0;padding:2px 12px;color:#57606a}table{border-collapse:collapse}
td,th{border:1px solid #d0d7de;padding:4px 10px}img{max-width:100%}ul.todo{list-style:none;padding-left:4px}
@media(prefers-color-scheme:dark){body{background:#0d1117;color:#e6edf3}pre,code{background:#161b22}td,th{border-color:#30363d}}`

func htmlPage(s *notes.Store, list []proto.Note) string {
	titles := map[string]string{} // a title → its section
	for _, n := range list {
		if _, ok := titles[strings.ToLower(n.Title)]; !ok {
			titles[strings.ToLower(n.Title)] = "n-" + n.ID
		}
	}
	var b strings.Builder
	title := "Notes"
	if len(list) == 1 {
		title = list[0].Title
	}
	fmt.Fprintf(&b, "<!doctype html>\n<html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width\">\n<title>%s</title><style>%s</style></head><body>\n",
		html.EscapeString(title), pageStyle)
	if len(list) > 1 {
		b.WriteString("<nav><h2>Contents</h2><ul>\n")
		for _, n := range list {
			fmt.Fprintf(&b, "<li><a href=\"#n-%s\">%s</a></li>\n", n.ID, html.EscapeString(iconTitle(n)))
		}
		b.WriteString("</ul></nav>\n")
	}
	for _, n := range list {
		fmt.Fprintf(&b, "<section id=\"n-%s\"><h1>%s</h1>\n<p class=\"meta\">Updated %s</p>\n", n.ID, html.EscapeString(iconTitle(n)), n.Updated.Format("2 Jan 2006"))
		b.WriteString(blocksHTML(s, parseMarkdown(n.Body), titles))
		b.WriteString("</section>\n")
	}
	b.WriteString("</body></html>\n")
	return b.String()
}

func iconTitle(n proto.Note) string {
	t := n.Title
	if t == "" {
		t = "Untitled"
	}
	if n.Icon != "" && !strings.Contains(n.Icon, ":") {
		t = n.Icon + " " + t
	}
	return t
}

func blocksHTML(s *notes.Store, blocks []mdBlock, titles map[string]string) string {
	var b strings.Builder
	var open []string // the lists open, innermost last
	closeTo := func(depth int, tag string) {
		for len(open) > depth+1 || depth >= 0 && len(open) == depth+1 && open[depth] != tag {
			b.WriteString("</" + strings.Fields(open[len(open)-1])[0] + ">\n")
			open = open[:len(open)-1]
		}
		for len(open) < depth+1 {
			if tag == "ul class=\"todo\"" {
				b.WriteString("<ul class=\"todo\">\n")
			} else {
				b.WriteString("<" + tag + ">\n")
			}
			open = append(open, tag)
		}
	}
	for _, bl := range blocks {
		switch bl.kind {
		case mdBullet:
			closeTo(bl.level, "ul")
			b.WriteString("<li>" + inlineHTML(s, bl.text, titles) + "</li>\n")
			continue
		case mdNumber:
			closeTo(bl.level, "ol")
			b.WriteString("<li>" + inlineHTML(s, bl.text, titles) + "</li>\n")
			continue
		case mdTodo:
			closeTo(bl.level, "ul class=\"todo\"")
			check := ""
			if bl.done {
				check = " checked"
			}
			b.WriteString("<li><input type=\"checkbox\" disabled" + check + "> " + inlineHTML(s, bl.text, titles) + "</li>\n")
			continue
		}
		closeTo(-1, "")
		switch bl.kind {
		case mdHeading:
			fmt.Fprintf(&b, "<h%d>%s</h%d>\n", bl.level+1, inlineHTML(s, bl.text, titles), bl.level+1)
		case mdQuote:
			b.WriteString("<blockquote>" + inlineHTML(s, bl.text, titles) + "</blockquote>\n")
		case mdCode:
			b.WriteString("<pre><code>" + html.EscapeString(bl.text) + "</code></pre>\n")
		case mdDivider:
			b.WriteString("<hr>\n")
		case mdImage:
			b.WriteString("<p>" + imageHTML(s, bl.src, bl.text) + "</p>\n")
		case mdTable:
			b.WriteString("<table>\n")
			for i, row := range bl.rows {
				tag := "td"
				if i == 0 {
					tag = "th"
				}
				b.WriteString("<tr>")
				for _, c := range row {
					b.WriteString("<" + tag + ">" + inlineHTML(s, c, titles) + "</" + tag + ">")
				}
				b.WriteString("</tr>\n")
			}
			b.WriteString("</table>\n")
		default:
			b.WriteString("<p>" + inlineHTML(s, bl.text, titles) + "</p>\n")
		}
	}
	closeTo(-1, "")
	return b.String()
}

// imageHTML is an image: a note's file put in the page, a web image by
// its address.
func imageHTML(s *notes.Store, src, alt string) string {
	if strings.HasPrefix(src, "assets/") {
		data, err := s.Asset(src)
		if err != nil {
			return html.EscapeString(alt)
		}
		return fmt.Sprintf(`<img alt="%s" src="data:%s;base64,%s">`, html.EscapeString(alt), http.DetectContentType(data), base64.StdEncoding.EncodeToString(data))
	}
	if u := safeURL(src); u != "" {
		return fmt.Sprintf(`<img alt="%s" src="%s">`, html.EscapeString(alt), html.EscapeString(u))
	}
	return html.EscapeString(alt)
}

// inlineHTML writes a line's marks as HTML, the rest escaped.
func inlineHTML(s *notes.Store, text string, titles map[string]string) string {
	// Each mark becomes a token first, so escaping leaves what it makes.
	var tokens []string
	token := func(h string) string {
		tokens = append(tokens, h)
		return fmt.Sprintf("\x00%d\x00", len(tokens)-1)
	}
	text = inlineCode.ReplaceAllStringFunc(text, func(m string) string {
		return token("<code>" + html.EscapeString(inlineCode.FindStringSubmatch(m)[1]) + "</code>")
	})
	text = inlineImage.ReplaceAllStringFunc(text, func(m string) string {
		sub := inlineImage.FindStringSubmatch(m)
		return token(imageHTML(s, sub[2], sub[1]))
	})
	text = inlineWiki.ReplaceAllStringFunc(text, func(m string) string {
		sub := inlineWiki.FindStringSubmatch(m)
		label := sub[1]
		if sub[3] != "" {
			label = sub[3][1:]
		}
		if id, ok := titles[strings.ToLower(strings.TrimSpace(sub[1]))]; ok {
			return token(`<a href="#` + id + `">` + html.EscapeString(label) + "</a>")
		}
		return label
	})
	text = inlineLink.ReplaceAllStringFunc(text, func(m string) string {
		sub := inlineLink.FindStringSubmatch(m)
		if u := safeURL(sub[2]); u != "" {
			return token(`<a href="` + html.EscapeString(u) + `">` + html.EscapeString(sub[1]) + "</a>")
		}
		return sub[1]
	})
	text = html.EscapeString(text)
	text = inlineBold.ReplaceAllString(text, "<strong>$1</strong>")
	text = inlineStrk.ReplaceAllString(text, "<del>$1</del>")
	text = inlineItal.ReplaceAllString(text, "<em>$1</em>")
	for i, t := range tokens {
		text = strings.Replace(text, fmt.Sprintf("\x00%d\x00", i), t, 1)
	}
	return text
}
