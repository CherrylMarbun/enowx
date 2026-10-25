package notesio

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// ApplyOptions say where an import's notes go and what becomes of notes
// made before from the same source.
type ApplyOptions struct {
	Group   string // the group the notes at the top go in ("" for their source's)
	Project string // the project they are for, kept with the import
	Mode    string // proto.ImportSkip (""), ImportOverwrite or ImportDuplicate
	// Progress is told of each note written, and how many there are.
	Progress func(done, total int)
}

// record is a note an import made, in imports.json beside the notes: its
// source's name for it, so importing that again finds it.
type record struct {
	Source   string    `json:"source"`
	Note     string    `json:"note"`
	Project  string    `json:"project,omitempty"`
	Imported time.Time `json:"imported"`
}

func recordsFile(s *notes.Store) string { return filepath.Join(s.Dir(), "imports.json") }

func loadRecords(s *notes.Store) ([]record, error) {
	data, err := os.ReadFile(recordsFile(s))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	var out []record
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("import: imports.json: %w", err)
	}
	return out, nil
}

func saveRecords(s *notes.Store, list []record) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir(), 0o700); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	tmp := recordsFile(s) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	return os.Rename(tmp, recordsFile(s))
}

// plan is what Apply does with each Doc: the note it writes and how.
type plan struct {
	ids     []string // each Doc's note
	actions []string // "new", or the mode's for a note made before
	titles  map[string]string
}

func newID() string {
	var b [6]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (b *Batch) plan(s *notes.Store, o ApplyOptions) (plan, error) {
	list, err := s.List()
	if err != nil {
		return plan{}, err
	}
	exists := map[string]bool{}
	for _, n := range list {
		exists[n.ID] = true
	}
	records, err := loadRecords(s)
	if err != nil {
		return plan{}, err
	}
	made := map[string]string{} // source → note
	for _, r := range records {
		if exists[r.Note] {
			made[r.Source] = r.Note
		}
	}
	p := plan{titles: map[string]string{}}
	for _, d := range b.Docs {
		id, action := "", "new"
		if old := made[d.Source]; old != "" && d.Source != "" {
			switch o.Mode {
			case proto.ImportOverwrite:
				id, action = old, proto.ImportOverwrite
			case proto.ImportDuplicate:
				action = proto.ImportDuplicate
			default:
				id, action = old, proto.ImportSkip
			}
		}
		if id == "" {
			id = d.ID
			if id == "" || exists[id] || slices.Contains(p.ids, id) {
				id = newID()
			}
		}
		p.ids = append(p.ids, id)
		p.actions = append(p.actions, action)
		p.titles[d.Key] = d.Title
	}
	return p, nil
}

// Preview says what Apply would make.
func (b *Batch) Preview(s *notes.Store, o ApplyOptions) (proto.NoteImportPreview, error) {
	p, err := b.plan(s, o)
	if err != nil {
		return proto.NoteImportPreview{}, err
	}
	list, err := s.List()
	if err != nil {
		return proto.NoteImportPreview{}, err
	}
	taken := map[string]bool{}
	for _, n := range list {
		if n.Parent == "" {
			taken[n.Group+"\x00"+strings.ToLower(n.Title)] = true
		}
	}
	out := proto.NoteImportPreview{From: b.From, Notes: []proto.NoteImportEntry{}}
	byKey := map[string]Doc{}
	for i, d := range b.Docs {
		byKey[d.Key] = d
		e := proto.NoteImportEntry{Title: d.Title, Group: b.groupOf(d, o), Action: p.actions[i]}
		for k := d.Parent; k != ""; k = byKey[k].Parent {
			e.Path = append([]string{byKey[k].Title}, e.Path...)
		}
		out.Notes = append(out.Notes, e)
		out.Assets += len(d.Assets)
		if d.Parent == "" && p.actions[i] != proto.ImportSkip && p.actions[i] != proto.ImportOverwrite && taken[e.Group+"\x00"+strings.ToLower(d.Title)] {
			out.Conflicts = append(out.Conflicts, d.Title)
		}
	}
	return out, nil
}

func (b *Batch) groupOf(d Doc, o ApplyOptions) string {
	switch {
	case d.Parent != "":
		return ""
	case o.Group != "":
		return o.Group
	}
	return d.Group
}

var (
	keyLink   = regexp.MustCompile(`\[\[([^\]\n|#]+)((?:#[^\]\n|]*)?)((?:\|[^\]\n]*)?)\]\]`)
	assetLink = regexp.MustCompile(`\]\(asset:([^)\n]+)\)`)
)

// Apply writes the batch's notes to s, parents first, and keeps what it
// made so importing the same source again finds them. Canceled, it takes
// back the notes it made.
func (b *Batch) Apply(ctx context.Context, s *notes.Store, o ApplyOptions) (proto.NoteImportReport, error) {
	report := proto.NoteImportReport{Skipped: slices.Clone(b.Skipped), Unconverted: slices.Clone(b.Unconverted)}
	p, err := b.plan(s, o)
	if err != nil {
		return report, err
	}
	for _, g := range b.Groups {
		_ = s.SaveGroup("", g) // a group there already stays as it is
	}
	records, err := loadRecords(s)
	if err != nil {
		return report, err
	}
	keyID := map[string]string{}
	for i, d := range b.Docs {
		keyID[d.Key] = p.ids[i]
	}
	var made []string
	undo := func() {
		for i := len(made) - 1; i >= 0; i-- {
			_ = s.Delete(made[i])
		}
	}
	now := time.Now().UTC().Truncate(time.Second)
	for i, d := range b.Docs {
		if err := ctx.Err(); err != nil {
			undo()
			return report, err
		}
		id, action := p.ids[i], p.actions[i]
		if o.Progress != nil {
			o.Progress(i, len(b.Docs))
		}
		if action == proto.ImportSkip {
			report.Skipped = append(report.Skipped, proto.NoteImportIssue{Where: d.Title, What: "imported before"})
			continue
		}
		body := keyLink.ReplaceAllStringFunc(d.Body, func(m string) string {
			sub := keyLink.FindStringSubmatch(m)
			key := strings.TrimSpace(sub[1])
			title, ok := p.titles[key]
			if !ok {
				return m
			}
			label := strings.TrimPrefix(sub[3], "|")
			if label == title {
				label = ""
			}
			return wikilink(title, strings.TrimPrefix(sub[2], "#"), label)
		})
		var assetErr error
		body = assetLink.ReplaceAllStringFunc(body, func(m string) string {
			name := assetLink.FindStringSubmatch(m)[1]
			for _, a := range d.Assets {
				if a.Name == name {
					ref, err := s.SaveAsset(id, a.Name, a.Data)
					if err != nil {
						assetErr = err
						return m
					}
					return "](" + ref + ")"
				}
			}
			return "]()"
		})
		if assetErr != nil {
			undo()
			return report, assetErr
		}
		n := proto.Note{NoteInfo: proto.NoteInfo{ID: id, Title: d.Title, Icon: d.Icon, Parent: keyID[d.Parent],
			Group: b.groupOf(d, o), Order: -1, Created: d.Created, Updated: d.Updated}, Body: body}
		if n.Updated.IsZero() {
			n.Updated = now
		}
		if action == proto.ImportOverwrite {
			if _, err := s.Replace(n); err != nil {
				return report, err
			}
			report.Overwritten++
		} else {
			info, err := s.Restore(n)
			if err != nil {
				undo()
				return report, fmt.Errorf("import: %s: %w", d.Title, err)
			}
			made = append(made, info.ID)
			report.Imported++
			if d.Parent == "" {
				report.Notes = append(report.Notes, info.ID)
			}
			records = slices.DeleteFunc(records, func(r record) bool { return r.Source == d.Source && action != proto.ImportDuplicate })
			if d.Source != "" {
				records = append(records, record{Source: d.Source, Note: info.ID, Project: o.Project, Imported: now})
			}
		}
	}
	if o.Progress != nil {
		o.Progress(len(b.Docs), len(b.Docs))
	}
	return report, saveRecords(s, records)
}
