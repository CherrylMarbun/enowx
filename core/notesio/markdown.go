package notesio

import (
	"context"
	"path"
	"strings"
	"unicode/utf8"
)

// An Obsidian vault, or any folder of Markdown: a folder at its top
// becomes a group, a folder deeper a page holding its notes (the note of
// the same name beside it, when there is one, as Obsidian's folder notes
// are); [[wikilinks]] and links to the folder's files become links to
// the notes made, the files they show the notes' own.

func isMarkdown(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown", ".txt":
		return true
	}
	return false
}

// commonRoot is the one folder every file is in, "" when there is none
// (a .zip of a vault holds its folder).
func commonRoot(files []string) string {
	root := ""
	for i, f := range files {
		top, _, ok := strings.Cut(f, "/")
		if !ok {
			return ""
		}
		if i == 0 {
			root = top
		} else if top != root {
			return ""
		}
	}
	return root
}

func readMarkdown(ctx context.Context, s *source, b *Batch) error {
	root := commonRoot(s.files)
	rel := func(f string) string {
		if root == "" {
			return f
		}
		return strings.TrimPrefix(f, root+"/")
	}
	vault := strings.TrimSuffix(s.name, path.Ext(s.name))
	if root != "" {
		vault = root
	}
	// The notes, by key: their path without the extension.
	var notes []string
	byKey := map[string]string{}    // key → file
	byBase := map[string]string{}   // a note's name → key, as wikilinks name them
	fileBase := map[string]string{} // a file's name → path, as ![[x.png]] names it
	for _, f := range s.files {
		r := rel(f)
		if strings.HasPrefix(r, ".obsidian/") || strings.HasPrefix(r, ".trash/") {
			continue
		}
		fileBase[strings.ToLower(path.Base(f))] = f
		if isMarkdown(f) {
			key := strings.TrimSuffix(r, path.Ext(r))
			notes = append(notes, f)
			byKey[key] = f
			if _, ok := byBase[strings.ToLower(path.Base(key))]; !ok {
				byBase[strings.ToLower(path.Base(key))] = key
			}
		}
	}
	// Folders holding notes become groups (at the top) or pages (deeper).
	folders := map[string]bool{}
	for key := range byKey {
		for d := path.Dir(key); d != "."; d = path.Dir(d) {
			folders[d] = true
		}
	}
	parentOf := func(key string) (parent, group string) {
		dir := path.Dir(key)
		if dir == "." {
			return "", ""
		}
		top, _, _ := strings.Cut(dir, "/")
		if !strings.Contains(dir, "/") {
			return "", top
		}
		return dir, top
	}
	single := len(notes) == 1 && len(s.files) == 1
	added := map[string]bool{}
	var addFolder func(dir string)
	addFolder = func(dir string) {
		if added[dir] || !strings.Contains(dir, "/") {
			return
		}
		added[dir] = true
		parent, group := parentOf(dir)
		addFolder(parent)
		if _, ok := byKey[dir]; ok {
			return // its folder note stands for it, added with the notes
		}
		b.Docs = append(b.Docs, Doc{Key: dir, Source: "md:" + vault + "/" + dir + "/", Parent: parent, Group: group, Title: path.Base(dir)})
	}
	l := &linker{src: s, b: b}
	l.note = func(dir, target string) (string, bool) {
		if isMarkdown(target) {
			target = strings.TrimSuffix(target, path.Ext(target))
		}
		if _, ok := byKey[path.Join(dir, target)]; ok {
			return path.Join(dir, target), true
		}
		if _, ok := byKey[target]; ok {
			return target, true
		}
		key, ok := byBase[strings.ToLower(path.Base(target))]
		return key, ok
	}
	l.file = func(dir, ref string) (string, bool) {
		full := ref
		if root != "" {
			full = root + "/" + path.Join(dir, ref)
		} else {
			full = path.Join(dir, ref)
		}
		if s.has(full) {
			return full, true
		}
		f, ok := fileBase[strings.ToLower(path.Base(ref))]
		return f, ok
	}
	for _, f := range notes {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := rel(f)
		key := strings.TrimSuffix(r, path.Ext(r))
		// A folder note (F/G.md beside F/G/) is its folder's page: the
		// notes in the folder name it as their parent's key already.
		parent, group := parentOf(key)
		addFolder(parent)
		data, err := s.read(f)
		if err != nil {
			b.skip(r, err.Error())
			continue
		}
		if !utf8.Valid(data) {
			b.skip(r, "not text")
			continue
		}
		meta, body := frontMatter(string(data))
		title := path.Base(key)
		if t := meta["title"]; t != "" {
			title = t
		}
		d := Doc{Key: key, Source: "md:" + vault + "/" + r, Parent: parent, Group: group, Title: title, Icon: meta["icon"],
			Created: metaTime(meta["created"]), Updated: metaTime(first(meta["updated"], meta["modified"]))}
		if single {
			d.Source, d.Group = "md:"+s.name, ""
		}
		l.doc = &d
		dir := path.Dir(r)
		if dir == "." {
			dir = ""
		}
		body = dropTitle(body, title)
		if tags := meta["tags"]; tags != "" {
			body = "Tags: " + tags + "\n\n" + body
		}
		d.Body = tidy(l.rewrite(body, dir))
		b.Docs = append(b.Docs, d)
	}
	orderParents(b)
	return nil
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// orderParents puts every Doc after the one it goes under.
func orderParents(b *Batch) {
	byKey := map[string]Doc{}
	for _, d := range b.Docs {
		byKey[d.Key] = d
	}
	var out []Doc
	done := map[string]bool{}
	var add func(d Doc)
	add = func(d Doc) {
		if done[d.Key] {
			return
		}
		done[d.Key] = true
		if p, ok := byKey[d.Parent]; ok {
			add(p)
		}
		out = append(out, d)
	}
	for _, d := range b.Docs {
		add(d)
	}
	b.Docs = out
}
