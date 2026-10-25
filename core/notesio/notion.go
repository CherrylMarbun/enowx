package notesio

import (
	"bytes"
	"context"
	"encoding/csv"
	"net/url"
	"path"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// A Notion export, as Markdown & CSV or as HTML, in a .zip or unpacked:
// each page a file named after its title and ID, the pages under it in
// a folder of the same name, a database a .csv beside the folder of its
// rows' pages.

// notionTitle is the title in a Notion file's name.
func notionTitle(name string) string {
	name = strings.TrimSuffix(name, path.Ext(name))
	if id := notionID(name); id != "" {
		name = strings.TrimSpace(strings.TrimSuffix(name, id))
	}
	return name
}

// notionParent is the ID of the page or database a file is under: its
// folder's.
func notionParent(f string) string {
	dir := path.Dir(f)
	if dir == "." {
		return ""
	}
	return notionID(path.Base(dir))
}

var asideRe = regexp.MustCompile(`(?s)<aside>\s*(.*?)\s*</aside>`)

func readNotion(ctx context.Context, s *source, b *Batch, o Options) error {
	pages := map[string]string{} // ID → file
	dbs := map[string]string{}   // a database's ID → its .csv
	for _, f := range s.files {
		id := notionID(path.Base(f))
		if id == "" {
			continue
		}
		switch strings.ToLower(path.Ext(f)) {
		case ".md", ".html":
			pages[id] = f
		case ".csv":
			// Newer exports write <name>_all.csv too, with every row: that one.
			if strings.HasSuffix(strings.TrimSuffix(f, ".csv"), "_all") || dbs[id] == "" {
				dbs[id] = f
			}
		}
	}
	for _, f := range s.files {
		if strings.HasSuffix(f, "_all.csv") {
			if id := notionID(strings.TrimSuffix(path.Base(f), "_all.csv")); id != "" {
				dbs[id] = f
			}
		}
	}
	parentOf := func(f string) string {
		p := notionParent(f)
		if _, ok := pages[p]; ok {
			return "notion:" + p
		}
		if _, ok := dbs[p]; ok {
			return "notion:" + p
		}
		return ""
	}
	// Databases first: which of their rows' pages are rows only.
	rowOnly := map[string]bool{}
	dbDocs := map[string]*Doc{}
	for id, f := range dbs {
		data, err := s.read(f)
		if err != nil {
			b.skip(f, err.Error())
			continue
		}
		rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\ufeff")))).ReadAll()
		if err != nil || len(rows) == 0 {
			b.skip(f, "not a table")
			continue
		}
		name := notionTitle(strings.TrimSuffix(path.Base(f), "_all.csv"))
		d := &Doc{Key: "notion:" + id, Source: "notion:" + id, Parent: parentOf(f), Title: name, Icon: "🗃️"}
		// Its rows' pages, by title: those with more than their properties
		// stay pages.
		rowPages := map[string]string{}
		for pid, pf := range pages {
			if notionParent(pf) == id {
				rowPages[notionTitle(path.Base(pf))] = pid
			}
		}
		head := rows[0]
		var lines [][]string
		lines = append(lines, head)
		var listing []string
		for _, r := range rows[1:] {
			if len(r) == 0 {
				continue
			}
			pid := rowPages[r[0]]
			if pid != "" {
				body, _ := s.read(pages[pid])
				if o.RowsAsNotes || hasOwnContent(string(body), head) {
					r[0] = wikilink("notion:"+pid, "", "")
					listing = append(listing, "- "+r[0])
				} else {
					rowOnly[pid] = true
				}
			}
			cells := make([]string, len(r))
			for i, c := range r {
				cells[i] = strings.ReplaceAll(strings.ReplaceAll(c, "|", `\|`), "\n", " ")
			}
			lines = append(lines, cells)
		}
		if o.RowsAsNotes {
			d.Body = strings.Join(listing, "\n") + "\n"
			for pid := range rowPages {
				delete(rowOnly, pid)
			}
		} else {
			d.Body = markdownTable(lines) + "\n"
		}
		dbDocs[id] = d
		b.Docs = append(b.Docs, *d)
	}
	l := &linker{src: s, b: b}
	l.note = func(dir, target string) (string, bool) {
		id := notionID(path.Base(target))
		if id == "" {
			if u, err := url.Parse(target); err == nil {
				id = notionID(path.Base(u.Path))
			}
		}
		if _, ok := pages[id]; ok && !rowOnly[id] {
			return "notion:" + id, true
		}
		if _, ok := dbs[id]; ok {
			return "notion:" + id, true
		}
		return "", false
	}
	l.file = func(dir, ref string) (string, bool) {
		p := path.Join(dir, ref)
		return p, s.has(p)
	}
	for _, f := range s.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		id := notionID(path.Base(f))
		if pages[id] != f || rowOnly[id] {
			continue
		}
		data, err := s.read(f)
		if err != nil {
			b.skip(f, err.Error())
			continue
		}
		d := Doc{Key: "notion:" + id, Source: "notion:" + id, Parent: parentOf(f), Title: notionTitle(path.Base(f))}
		l.doc = &d
		dir := path.Dir(f)
		if strings.EqualFold(path.Ext(f), ".html") {
			notionHTML(&d, string(data), dir, l, b)
		} else {
			text := strings.ReplaceAll(string(data), "\r\n", "\n")
			if h, rest, _ := strings.Cut(text, "\n"); strings.HasPrefix(h, "# ") {
				d.Title, text = strings.TrimSpace(h[2:]), rest
			}
			if db := dbs[notionParent(f)]; db != "" {
				text = dropProperties(text, dbHead(s, db))
			}
			text = asideRe.ReplaceAllStringFunc(text, func(m string) string {
				inner := asideRe.FindStringSubmatch(m)[1]
				return "> " + strings.ReplaceAll(strings.TrimSpace(inner), "\n", "\n> ")
			})
			d.Body = tidy(l.rewrite(text, dir))
		}
		b.Docs = append(b.Docs, d)
	}
	orderParents(b)
	return nil
}

// dbHead is a database's columns.
func dbHead(s *source, f string) []string {
	data, err := s.read(f)
	if err != nil {
		return nil
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\ufeff"))))
	head, _ := r.Read()
	return head
}

// propertyLines splits a row page's text: its property lines ("Status:
// Done"), which the table has, and the rest.
func propertyLines(text string, head []string) (props []string, rest string) {
	cols := map[string]bool{}
	for _, h := range head {
		cols[h] = true
	}
	lines := strings.Split(strings.TrimLeft(text, "\n"), "\n")
	i := 0
	for ; i < len(lines); i++ {
		k, _, ok := strings.Cut(lines[i], ":")
		if !ok || !cols[k] {
			break
		}
		props = append(props, lines[i])
	}
	return props, strings.Join(lines[i:], "\n")
}

func dropProperties(text string, head []string) string {
	props, rest := propertyLines(text, head)
	if len(props) == 0 {
		return text
	}
	return "Properties: " + strings.Join(props, " · ") + "\n\n" + strings.TrimLeft(rest, "\n")
}

// hasOwnContent says whether a row's page holds more than its title and
// properties.
func hasOwnContent(text string, head []string) bool {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if h, rest, _ := strings.Cut(text, "\n"); strings.HasPrefix(h, "<") || strings.HasPrefix(h, "# ") {
		if strings.HasPrefix(h, "<") {
			return true // an HTML page: kept as a page
		}
		text = rest
	}
	_, rest := propertyLines(text, head)
	return strings.TrimSpace(rest) != ""
}

// notionHTML reads a page of Notion's HTML export into d.
func notionHTML(d *Doc, src, dir string, l *linker, b *Batch) {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		b.skip(d.Title, "unreadable HTML")
		return
	}
	if t := findClass(doc, "page-title"); t != nil {
		d.Title = strings.TrimSpace(rawText(t))
	}
	if ic := findClass(doc, "page-header-icon"); ic != nil {
		if span := find(ic, atom.Span); span != nil {
			d.Icon = strings.TrimSpace(rawText(span))
		}
	}
	body := findClass(doc, "page-body")
	if body == nil {
		body = find(doc, atom.Body)
	}
	if body == nil {
		return
	}
	c := &htmlConv{batch: b, where: d.Title}
	c.link = func(href string) (string, bool) {
		if key, ok := l.note(dir, href); ok {
			return wikilink(key, "", ""), true
		}
		return "", false
	}
	c.image = func(src, alt string) string {
		if u := safeURL(src); u != "" {
			return "![" + alt + "](" + u + ")"
		}
		ref, err := url.PathUnescape(src)
		if err != nil {
			ref = src
		}
		if name, ok := l.asset(dir, ref); ok {
			return "![" + alt + "](asset:" + name + ")"
		}
		return ""
	}
	c.blocks(body, "")
	d.Body = tidy(c.out.String())
}
