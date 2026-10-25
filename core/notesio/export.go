package notesio

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/iconlib"
	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// Notes taken out of enowX: one as Markdown, several as a .zip of
// folders of Markdown with their files (the layout Obsidian opens as a
// vault and Notion imports), one HTML page or PDF, or a JSON backup. A
// .zip and a backup carry what enowX needs to make the notes again as
// they were: IDs, groups, order, icons, times.

// Selection picks the notes to export: IDs (with the notes under them),
// a group's, or all.
type Selection struct {
	IDs   []string
	Group string
	All   bool
}

// backupManifest is the file in an export's .zip that describes it.
const backupManifest = "enowx-notes.json"

const backupKind = "notes-backup"

// backup is an export's description, and a JSON backup whole.
type backup struct {
	Enowx    string            `json:"enowx"`
	Version  int               `json:"version"`
	Exported time.Time         `json:"exported"`
	Groups   []proto.NoteGroup `json:"groups,omitempty"`
	Notes    []backupNote      `json:"notes"`
}

type backupNote struct {
	proto.Note
	// Assets are the note's files: their data in a JSON backup, their
	// paths in a .zip's description.
	Assets map[string][]byte `json:"assets,omitempty"`
	Files  map[string]string `json:"files,omitempty"`
	File   string            `json:"file,omitempty"` // the note's .md in a .zip
}

// collect is the notes sel picks, in the tree's order, with their bodies,
// and the groups they are in.
func collect(s *notes.Store, sel Selection) ([]proto.Note, []proto.NoteGroup, error) {
	list, err := s.List()
	if err != nil {
		return nil, nil, err
	}
	children := map[string][]proto.NoteInfo{}
	for _, n := range list {
		children[n.Parent] = append(children[n.Parent], n)
	}
	picked := map[string]bool{}
	var mark func(id string)
	mark = func(id string) {
		picked[id] = true
		for _, c := range children[id] {
			mark(c.ID)
		}
	}
	for _, n := range list {
		switch {
		case sel.All, sel.Group != "" && n.Parent == "" && n.Group == sel.Group, slices.Contains(sel.IDs, n.ID):
			mark(n.ID)
		}
	}
	for _, id := range sel.IDs {
		if !picked[id] {
			return nil, nil, fmt.Errorf("export: no note %q", id)
		}
	}
	var out []proto.Note
	var walk func(parent string)
	walk = func(parent string) {
		for _, n := range children[parent] {
			if picked[n.ID] {
				full, err := s.Get(n.ID)
				if err == nil {
					out = append(out, full)
				}
			}
			walk(n.ID)
		}
	}
	walk("")
	if len(out) == 0 {
		return nil, nil, errors.New("export: no notes to export")
	}
	groups, err := s.Groups()
	if err != nil {
		return nil, nil, err
	}
	used := map[string]bool{}
	for _, n := range out {
		used[n.Group] = true
	}
	groups = slices.DeleteFunc(groups, func(g proto.NoteGroup) bool { return !used[g.Name] })
	return out, groups, nil
}

// Export writes the notes sel picks to w in format (proto.Export*), and
// says a name for the file and how many notes it holds.
func Export(s *notes.Store, sel Selection, format string, w io.Writer) (name string, count int, err error) {
	list, groups, err := collect(s, sel)
	if err != nil {
		return "", 0, err
	}
	base := "enowx-notes"
	switch {
	case sel.Group != "":
		base = fileName(sel.Group)
	case len(sel.IDs) == 1:
		base = fileName(list[0].Title)
	}
	switch format {
	case proto.ExportMarkdown, "":
		if len(list) == 1 {
			_, err = io.WriteString(w, markdownFile(list[0], func(ref string) string { return ref }))
			return base + ".md", 1, err
		}
		return base + ".zip", len(list), writeZip(s, list, groups, w)
	case proto.ExportZip:
		return base + ".zip", len(list), writeZip(s, list, groups, w)
	case proto.ExportJSON:
		b := backup{Enowx: backupKind, Version: 1, Exported: time.Now().UTC().Truncate(time.Second), Groups: groups}
		for _, n := range list {
			bn := backupNote{Note: n}
			for _, ref := range assetRefs(n) {
				if data, err := s.Asset(ref); err == nil {
					if bn.Assets == nil {
						bn.Assets = map[string][]byte{}
					}
					bn.Assets[path.Base(ref)] = data
				}
			}
			b.Notes = append(b.Notes, bn)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return base + ".json", len(list), enc.Encode(b)
	case proto.ExportHTML:
		_, err = io.WriteString(w, htmlPage(s, list))
		return base + ".html", len(list), err
	case proto.ExportPDF:
		return base + ".pdf", len(list), writePDF(s, list, w)
	}
	return "", 0, fmt.Errorf("export: no format %q (md, zip, html, pdf, json)", format)
}

var assetRef = regexp.MustCompile(`\]\((assets/[0-9a-f]{12}/[^)\s]+)\)`)

// assetRefs are the files a note's body shows.
func assetRefs(n proto.Note) []string {
	var out []string
	for _, m := range assetRef.FindAllStringSubmatch(n.Body, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// fileName is a title made a file's name.
func fileName(title string) string {
	name := strings.Map(func(r rune) rune {
		if r < ' ' || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(title))
	name = strings.Trim(name, ". ")
	if name == "" {
		return "Untitled"
	}
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
	}
	return name
}

// markdownFile is a note as a .md: its front matter, then its body, its
// files named by fix.
func markdownFile(n proto.Note, fix func(ref string) string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", yamlValue(n.Title))
	if n.Icon != "" {
		fmt.Fprintf(&b, "icon: %s\n", yamlValue(n.Icon))
		// The icon itself, for editors that don't know enowX's libraries.
		if uri := iconlib.IconDataURI(n.Icon); uri != "" {
			fmt.Fprintf(&b, "icon-svg: %s\n", yamlValue(uri))
		}
	}
	if n.Group != "" {
		fmt.Fprintf(&b, "group: %s\n", yamlValue(n.Group))
	}
	if len(n.Projects) > 0 {
		var names []string
		for _, p := range n.Projects {
			name := p.Name
			if p.HostName != "" {
				name += " · " + p.HostName
			}
			names = append(names, yamlValue(name))
		}
		fmt.Fprintf(&b, "projects: [%s]\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(&b, "created: %s\n", n.Created.Format(time.RFC3339))
	fmt.Fprintf(&b, "updated: %s\n", n.Updated.Format(time.RFC3339))
	fmt.Fprintf(&b, "enowx: %s\n", n.ID)
	b.WriteString("---\n\n")
	// Icons in the text and drawings shown as images elsewhere.
	b.WriteString(assetRef.ReplaceAllStringFunc(iconlib.ExportMarkdown(n.Body), func(m string) string {
		return "](" + fix(assetRef.FindStringSubmatch(m)[1]) + ")"
	}))
	return b.String()
}

func yamlValue(s string) string {
	if s == "" || strings.ContainsAny(s, ":#'\"[]{}&*!|>%@`") || strings.TrimSpace(s) != s {
		data, _ := json.Marshal(s)
		return string(data)
	}
	return s
}

// writeZip writes notes as folders of Markdown: a group's in its folder,
// the notes under a note in a folder of its name, files in assets/.
func writeZip(s *notes.Store, list []proto.Note, groups []proto.NoteGroup, w io.Writer) error {
	zw := zip.NewWriter(w)
	byID := map[string]proto.Note{}
	for _, n := range list {
		byID[n.ID] = n
	}
	dirOf := map[string]string{} // a note's folder for the notes under it
	used := map[string]bool{}
	manifest := backup{Enowx: backupKind, Version: 1, Exported: time.Now().UTC().Truncate(time.Second), Groups: groups}
	for _, n := range list {
		dir := ""
		if p, ok := byID[n.Parent]; ok {
			dir = dirOf[p.ID]
		} else if n.Group != "" {
			dir = fileName(n.Group)
		}
		name := fileName(n.Title)
		file := path.Join(dir, name+".md")
		for i := 2; used[strings.ToLower(file)]; i++ {
			name = fmt.Sprintf("%s (%d)", fileName(n.Title), i)
			file = path.Join(dir, name+".md")
		}
		used[strings.ToLower(file)] = true
		dirOf[n.ID] = path.Join(dir, name)
		bn := backupNote{Note: n, File: file}
		up := strings.Repeat("../", strings.Count(file, "/"))
		for _, ref := range assetRefs(n) {
			data, err := s.Asset(ref)
			if err != nil {
				continue
			}
			fw, err := zw.Create(ref)
			if err != nil {
				return err
			}
			if _, err := fw.Write(data); err != nil {
				return err
			}
			if bn.Files == nil {
				bn.Files = map[string]string{}
			}
			bn.Files[path.Base(ref)] = ref
		}
		fw, err := zw.Create(file)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(fw, markdownFile(n, func(ref string) string { return up + ref })); err != nil {
			return err
		}
		manifest.Notes = append(manifest.Notes, bn)
	}
	fw, err := zw.Create(backupManifest)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(fw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return err
	}
	return zw.Close()
}

// readBackup reads an enowX export's .zip or a JSON backup: the notes as
// they were, under their own IDs where those are free.
func readBackup(s *source, b *Batch) error {
	file := backupManifest
	if !s.has(file) {
		if len(s.files) != 1 {
			return errors.New("import: no " + backupManifest + " in " + s.name)
		}
		file = s.files[0]
	} else {
		for _, f := range s.files {
			if path.Base(f) == backupManifest {
				file = f
			}
		}
	}
	data, err := s.read(file)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}
	var bk backup
	if err := json.Unmarshal(data, &bk); err != nil || bk.Enowx != backupKind {
		return errors.New("import: " + s.name + " isn't an enowX notes backup")
	}
	root := path.Dir(file)
	b.Groups = bk.Groups
	for _, n := range bk.Notes {
		d := Doc{Key: n.ID, Source: "enowx:" + n.ID, ID: n.ID, Parent: n.Parent, Group: n.Group, Title: n.Title, Icon: n.Icon,
			Created: n.Created, Updated: n.Updated}
		for name, data := range n.Assets {
			d.Assets = append(d.Assets, Asset{Name: name, Data: data})
		}
		for name, p := range n.Files {
			data, err := s.read(path.Join(root, p))
			if err != nil {
				b.skip(p, err.Error())
				continue
			}
			d.Assets = append(d.Assets, Asset{Name: name, Data: data})
		}
		slices.SortFunc(d.Assets, func(x, y Asset) int { return strings.Compare(x.Name, y.Name) })
		d.Body = assetRef.ReplaceAllStringFunc(n.Body, func(m string) string {
			return "](asset:" + path.Base(assetRef.FindStringSubmatch(m)[1]) + ")"
		})
		// Links between notes are by title already: as they are.
		b.Docs = append(b.Docs, d)
	}
	orderParents(b)
	return nil
}
