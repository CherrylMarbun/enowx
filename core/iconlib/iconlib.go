// Package iconlib is the SVG icons notes, groups and inline text use:
// libraries of icons kept in the binary (no network), searched by name and
// keyword, and the icon values notes store, "lucide:rocket" for a
// library's icon, "svg:<base64>" for one of the user's own.
package iconlib

import (
	"bytes"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
)

//go:embed data/*.json.gz
var data embed.FS

// Library is a set of icons drawn in one style.
type Library struct {
	ID, Name string
}

// Libraries are the libraries kept, the default first.
var Libraries = []Library{
	{"lucide", "Lucide"},
	{"tabler", "Tabler"},
	{"phosphor", "Phosphor"},
}

// Default is the icon of a note that has none of its own.
const Default = "lucide:file-text"

// Icon is a library's icon: its name and the words it is found by.
type Icon struct {
	Lib  string   `json:"lib"`
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

// Ref is the icon's value: "lib:name".
func (i Icon) Ref() string { return i.Lib + ":" + i.Name }

type entry struct {
	T []string `json:"t"`
	S string   `json:"s"`
}

type library struct {
	once  sync.Once
	icons map[string]entry
	names []string // sorted
	err   error
}

var libs = map[string]*library{}

func init() {
	for _, l := range Libraries {
		libs[l.ID] = &library{}
	}
}

// load reads library id once.
func load(id string) (*library, error) {
	l := libs[id]
	if l == nil {
		return nil, fmt.Errorf("iconlib: no library %q (there are %s)", id, libraryIDs())
	}
	l.once.Do(func() {
		raw, err := data.ReadFile("data/" + id + ".json.gz")
		if err != nil {
			l.err = err
			return
		}
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			l.err = err
			return
		}
		plain, err := io.ReadAll(zr)
		if err != nil {
			l.err = err
			return
		}
		if err := json.Unmarshal(plain, &l.icons); err != nil {
			l.err = err
			return
		}
		for name := range l.icons {
			l.names = append(l.names, name)
		}
		sort.Strings(l.names)
	})
	return l, l.err
}

func libraryIDs() string {
	var ids []string
	for _, l := range Libraries {
		ids = append(ids, l.ID)
	}
	return strings.Join(ids, ", ")
}

// Lookup is library lib's icon name as SVG.
func Lookup(lib, name string) (string, bool) {
	l, err := load(lib)
	if err != nil {
		return "", false
	}
	e, ok := l.icons[name]
	return e.S, ok
}

// All is library lib's icons, by name.
func All(lib string) []Icon {
	l, err := load(lib)
	if err != nil {
		return nil
	}
	out := make([]Icon, 0, len(l.names))
	for _, n := range l.names {
		out = append(out, Icon{Lib: lib, Name: n, Tags: l.icons[n].T})
	}
	return out
}

// Search is the icons of libs (nil: all) query names, best first, at most
// limit (0: no limit): a name equal to a word of it first, then names
// starting with it, holding it, then keywords holding it.
func Search(query string, libs []string, limit int) []Icon {
	q := strings.ToLower(strings.TrimSpace(query))
	q = strings.ReplaceAll(q, " ", "-")
	if len(libs) == 0 {
		for _, l := range Libraries {
			libs = append(libs, l.ID)
		}
	}
	type scored struct {
		Icon
		score int
	}
	var found []scored
	for li, id := range libs {
		l, err := load(id)
		if err != nil {
			continue
		}
		for _, name := range l.names {
			e := l.icons[name]
			s := score(q, name, e.T)
			if s == 0 {
				continue
			}
			// Libraries in the order asked break ties.
			found = append(found, scored{Icon{Lib: id, Name: name, Tags: e.T}, s*16 - li})
		}
	}
	slices.SortStableFunc(found, func(a, b scored) int {
		if a.score != b.score {
			return b.score - a.score
		}
		if len(a.Name) != len(b.Name) {
			return len(a.Name) - len(b.Name)
		}
		return strings.Compare(a.Name, b.Name)
	})
	out := make([]Icon, 0, len(found))
	for _, f := range found {
		if limit > 0 && len(out) == limit {
			break
		}
		out = append(out, f.Icon)
	}
	return out
}

func score(q, name string, tags []string) int {
	switch {
	case q == "":
		return 1
	case name == q:
		return 6
	case strings.HasPrefix(name, q+"-") || strings.HasSuffix(name, "-"+q):
		return 5
	case strings.HasPrefix(name, q):
		return 4
	case strings.Contains(name, q):
		return 3
	}
	tq := strings.ReplaceAll(q, "-", " ")
	for _, t := range tags {
		if strings.EqualFold(t, tq) {
			return 2
		}
	}
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), tq) {
			return 1
		}
	}
	return 0
}
