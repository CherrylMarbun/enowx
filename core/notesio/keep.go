package notesio

import (
	"context"
	"encoding/json"
	"net/url"
	"path"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Google Keep, from Google Takeout: a .json per note in Takeout/Keep, its
// pictures beside it; and HTML files of any kind, Apple Notes' exported
// among them, a note each, the folder at their top their group.

type keepNote struct {
	Title string `json:"title"`
	Text  string `json:"textContent"`
	List  []struct {
		Text    string `json:"text"`
		Checked bool   `json:"isChecked"`
	} `json:"listContent"`
	Attachments []struct {
		Path string `json:"filePath"`
		Mime string `json:"mimetype"`
	} `json:"attachments"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Trashed  bool  `json:"isTrashed"`
	Archived bool  `json:"isArchived"`
	Created  int64 `json:"createdTimestampUsec"`
	Edited   int64 `json:"userEditedTimestampUsec"`
}

func usec(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMicro(v).UTC()
}

func readKeep(ctx context.Context, s *source, b *Batch) error {
	for _, f := range s.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !strings.EqualFold(path.Ext(f), ".json") {
			continue
		}
		data, err := s.read(f)
		if err != nil {
			b.skip(f, err.Error())
			continue
		}
		var n keepNote
		if err := json.Unmarshal(data, &n); err != nil || n.Text == "" && len(n.List) == 0 && n.Title == "" && len(n.Attachments) == 0 {
			continue // Takeout's other files
		}
		if n.Trashed {
			b.skip(path.Base(f), "in Keep's trash")
			continue
		}
		title := strings.TrimSpace(n.Title)
		if title == "" {
			title = firstWords(n.Text)
		}
		d := Doc{Key: f, Source: "keep:" + path.Base(f), Title: title, Created: usec(n.Created), Updated: usec(n.Edited)}
		var body strings.Builder
		var tags []string
		for _, l := range n.Labels {
			tags = append(tags, l.Name)
		}
		if n.Archived {
			tags = append(tags, "Archived")
		}
		if len(tags) > 0 {
			body.WriteString("Tags: " + strings.Join(tags, ", ") + "\n\n")
		}
		if n.Text != "" {
			body.WriteString(strings.TrimSpace(n.Text) + "\n")
		}
		for _, item := range n.List {
			mark := " "
			if item.Checked {
				mark = "x"
			}
			body.WriteString("- [" + mark + "] " + strings.TrimSpace(item.Text) + "\n")
		}
		for _, a := range n.Attachments {
			p := path.Join(path.Dir(f), a.Path)
			data, err := s.read(p)
			if err != nil {
				b.skip(a.Path, "attachment not in the export")
				continue
			}
			d.Assets = append(d.Assets, Asset{Name: path.Base(p), Data: data})
			if strings.HasPrefix(a.Mime, "image/") {
				body.WriteString("\n![](asset:" + path.Base(p) + ")\n")
			} else {
				body.WriteString("\n[" + path.Base(p) + "](asset:" + path.Base(p) + ")\n")
			}
		}
		d.Body = tidy(body.String())
		b.Docs = append(b.Docs, d)
	}
	return nil
}

// firstWords titles a note without one: its first line, cut short.
func firstWords(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if r := []rune(line); len(r) > 60 {
		line = string(r[:60]) + "…"
	}
	if line == "" {
		return "Untitled"
	}
	return line
}

func isHTML(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return true
	}
	return false
}

func readHTML(ctx context.Context, s *source, b *Batch) error {
	root := commonRoot(s.files)
	keys := map[string]bool{}
	for _, f := range s.files {
		if isHTML(f) {
			keys[f] = true
		}
	}
	l := &linker{src: s, b: b}
	l.note = func(dir, target string) (string, bool) {
		p := path.Join(dir, target)
		return p, keys[p]
	}
	l.file = func(dir, ref string) (string, bool) {
		p := path.Join(dir, ref)
		return p, s.has(p)
	}
	for _, f := range s.files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !keys[f] {
			continue
		}
		data, err := s.read(f)
		if err != nil {
			b.skip(f, err.Error())
			continue
		}
		rel := strings.TrimPrefix(f, root+"/")
		group := ""
		if top, _, ok := strings.Cut(rel, "/"); ok {
			group = top
		}
		d := Doc{Key: f, Source: "html:" + s.name + "/" + rel, Group: group, Title: strings.TrimSuffix(path.Base(f), path.Ext(f))}
		if doc, err := html.Parse(strings.NewReader(string(data))); err == nil {
			if t := find(doc, atom.Title); t != nil && strings.TrimSpace(rawText(t)) != "" {
				d.Title = strings.TrimSpace(rawText(t))
			} else if h := find(doc, atom.H1); h != nil && strings.TrimSpace(rawText(h)) != "" {
				d.Title = strings.TrimSpace(rawText(h))
			}
		}
		l.doc = &d
		dir := path.Dir(f)
		c := &htmlConv{batch: b, where: d.Title}
		c.link = func(href string) (string, bool) {
			ref, err := url.PathUnescape(href)
			if err != nil || strings.Contains(href, ":") {
				return "", false
			}
			if key, ok := l.note(dir, ref); ok {
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
				return ""
			}
			if name, ok := l.asset(dir, ref); ok {
				return "![" + alt + "](asset:" + name + ")"
			}
			return ""
		}
		d.Body = dropTitle(c.convert(string(data)), d.Title)
		if len(s.files) == 1 {
			d.Group = ""
		}
		b.Docs = append(b.Docs, d)
	}
	return nil
}
