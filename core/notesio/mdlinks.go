package notesio

import (
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

// Links and files in Markdown read from a source: links to its other
// notes become [[key]] links, files they show become the note's assets.

var (
	mdLink   = regexp.MustCompile(`(!?)\[([^\]\n]*)\]\((<[^>\n]+>|[^)\s]+)(?:\s+"[^"\n]*")?\)`)
	wikiLink = regexp.MustCompile(`(!?)\[\[([^\]\n]+)\]\]`)
)

// linker resolves what a note's Markdown points at.
type linker struct {
	// note is the key of the note target names, from the note at dir.
	note func(dir, target string) (string, bool)
	// file is the path in the source of the file ref names, from dir.
	file func(dir, ref string) (string, bool)
	src  *source
	doc  *Doc
	b    *Batch
}

// rewrite resolves body's links and files, outside code. dir is the
// note's folder in the source.
func (l *linker) rewrite(body, dir string) string {
	lines := strings.Split(body, "\n")
	fence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fence = !fence
			continue
		}
		if fence {
			continue
		}
		parts := strings.Split(line, "`")
		for j := 0; j < len(parts); j += 2 { // the parts outside `code`
			parts[j] = wikiLink.ReplaceAllStringFunc(parts[j], func(m string) string { return l.wiki(m, dir) })
			parts[j] = mdLink.ReplaceAllStringFunc(parts[j], func(m string) string { return l.md(m, dir) })
		}
		lines[i] = strings.Join(parts, "`")
	}
	return strings.Join(lines, "\n")
}

// wiki resolves [[target#anchor|label]] and ![[file]].
func (l *linker) wiki(m, dir string) string {
	sub := wikiLink.FindStringSubmatch(m)
	embed, inner := sub[1] == "!", sub[2]
	target, label, _ := strings.Cut(inner, "|")
	target, anchor, _ := strings.Cut(target, "#")
	target = strings.TrimSpace(target)
	if embed {
		if name, ok := l.asset(dir, target); ok {
			return "![" + label + "](asset:" + name + ")"
		}
	}
	if target == "" {
		return m // a link to a heading of the same note
	}
	key, ok := l.note(dir, target)
	if !ok {
		return m // a note not in the source: by its title, if one has it
	}
	if embed {
		l.b.unconverted(l.doc.Title, "an embedded note, kept as a link")
	}
	return wikilink(key, anchor, label)
}

// md resolves [label](path) and ![alt](path) to the source's files.
func (l *linker) md(m, dir string) string {
	sub := mdLink.FindStringSubmatch(m)
	image, label, ref := sub[1] == "!", sub[2], strings.Trim(sub[3], "<>")
	if strings.HasPrefix(ref, "asset:") {
		return m // resolved already, from a wikilink
	}
	if u, err := url.Parse(ref); err == nil && u.Scheme != "" {
		if safeURL(ref) == "" {
			return label // javascript: and the like
		}
		return m
	}
	ref, anchor, _ := strings.Cut(ref, "#")
	if dec, err := url.PathUnescape(ref); err == nil {
		ref = dec
	}
	if ref == "" {
		return m
	}
	ext := strings.ToLower(path.Ext(ref))
	if !image && (ext == ".md" || ext == ".html" || ext == ".txt" || ext == ".csv" || ext == "") {
		if key, ok := l.note(dir, ref); ok {
			return wikilink(key, anchor, label)
		}
	}
	if name, ok := l.asset(dir, ref); ok {
		if image {
			return "![" + label + "](asset:" + name + ")"
		}
		return "[" + label + "](asset:" + name + ")"
	}
	if image {
		l.b.unconverted(l.doc.Title, "an image not in the import: "+path.Base(ref))
		return label
	}
	return label
}

// asset adds the file ref names to the note's assets, and says its name.
func (l *linker) asset(dir, ref string) (string, bool) {
	p, ok := l.file(dir, ref)
	if !ok {
		return "", false
	}
	name := path.Base(p)
	for _, a := range l.doc.Assets {
		if a.Name == name {
			return name, true
		}
	}
	data, err := l.src.read(p)
	if err != nil {
		l.b.skip(p, err.Error())
		return "", false
	}
	l.doc.Assets = append(l.doc.Assets, Asset{Name: name, Data: data})
	return name, true
}

// wikilink writes a link to the note key: [[key#anchor|label]].
func wikilink(key, anchor, label string) string {
	s := key
	if anchor != "" {
		s += "#" + anchor
	}
	if label = strings.TrimSpace(label); label != "" && label != key {
		s += "|" + label
	}
	return "[[" + s + "]]"
}

// frontMatter splits YAML front matter off a Markdown file: its simple
// "key: value" lines, and the body after it.
func frontMatter(text string) (map[string]string, string) {
	text = strings.TrimPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "\ufeff")
	if !strings.HasPrefix(text, "---\n") {
		return nil, text
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil, text
	}
	head, body := text[4:4+end], text[4+end+4:]
	body = strings.TrimPrefix(body, "\n")
	meta := map[string]string{}
	var listKey string
	for _, line := range strings.Split(head, "\n") {
		if item, ok := strings.CutPrefix(strings.TrimSpace(line), "- "); ok && listKey != "" {
			if meta[listKey] != "" {
				meta[listKey] += ", "
			}
			meta[listKey] += strings.Trim(strings.TrimSpace(item), `"'`)
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		value = strings.Trim(value, "[]")
		meta[key], listKey = value, key
	}
	return meta, body
}

// metaTime reads a front matter's date.
func metaTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Time{}
}

// dropTitle drops a body's first heading when it repeats the title.
func dropTitle(body, title string) string {
	trimmed := strings.TrimLeft(body, "\n")
	first, rest, _ := strings.Cut(trimmed, "\n")
	if h, ok := strings.CutPrefix(first, "# "); ok && strings.TrimSpace(h) == strings.TrimSpace(title) {
		return strings.TrimLeft(rest, "\n")
	}
	return body
}
