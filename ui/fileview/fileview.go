// Package fileview is the side panel's Files tool: the tree of a session's
// folder, with what git says changed, and a file's text. The daemon reads
// the folder, so it works the same for a session on a server; the view
// reads the folders open again every few seconds while it shows, since the
// agent keeps changing them.
package fileview

import (
	"path"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// refreshEvery is how often a shown view reads its open folders again.
const refreshEvery = 4 * time.Second

// Options connect a view to the app.
type Options struct {
	Client  *client.Client
	Session proto.SessionInfo
	// Update runs a function on the main thread; Wanted, called there,
	// reports whether the view still shows, else its poller stops.
	Update func(func())
	Wanted func() bool
	// OpenInEditor opens an absolute path of the host at a line (0 for
	// none) in the editor named EditorName.
	OpenInEditor func(path string, line int)
	EditorName   string
}

// View is one session's file tree and the file open. It belongs to the
// main thread; its poller reports through Options.Update.
type View struct {
	root    string                       // the session's folder, absolute
	dirs    map[string]proto.FileListing // folders read, by path ("" is the root)
	open    map[string]bool              // folders unfolded
	err     string
	loaded  bool
	polling bool
	refresh chan struct{} // asks the poller to read now

	rows []row
	list ui.ListState

	file     string // the path of the file shown, "" for the tree
	show     string // a file to show on the next Draw (Show)
	content  *proto.FileContent
	reading  bool
	fileList ui.ListState
	lines    []string
}

// New returns a view with nothing read.
func New() *View {
	return &View{dirs: map[string]proto.FileListing{}, open: map[string]bool{"": true}, refresh: make(chan struct{}, 1)}
}

// follow reads the open folders while the view is wanted, unless it
// already does.
func (v *View) follow(o Options) {
	if v.polling || o.Client == nil {
		return
	}
	v.polling = true
	cl, id := o.Client, o.Session.ID
	go func() {
		for {
			var paths []string
			keep := make(chan bool, 1)
			o.Update(func() {
				if !o.Wanted() {
					v.polling = false
					keep <- false
					return
				}
				for p := range v.open {
					paths = append(paths, p)
				}
				keep <- true
			})
			if !<-keep {
				return
			}
			read := map[string]proto.FileListing{}
			var failed error
			for _, p := range paths {
				l, err := cl.ListFiles(id, p)
				if err != nil {
					if p == "" {
						failed = err
					}
					continue
				}
				read[p] = l
			}
			o.Update(func() {
				v.loaded = true
				v.err = ""
				if failed != nil {
					v.err = failed.Error()
				}
				for p, l := range read {
					v.dirs[p] = l
					v.root = l.Root
				}
				v.layout()
			})
			select {
			case <-time.After(refreshEvery):
			case <-v.refresh:
			}
		}
	}()
}

// reload asks the poller to read now.
func (v *View) reload() {
	select {
	case v.refresh <- struct{}{}:
	default:
	}
}

// toggle unfolds the folder at p, reading it, or folds it.
func (v *View) toggle(p string) {
	if v.open[p] {
		delete(v.open, p)
	} else {
		v.open[p] = true
		v.reload()
	}
	v.layout()
}

// foldAll folds every folder but the root.
func (v *View) foldAll() {
	v.open = map[string]bool{"": true}
	v.layout()
}

// row is a row of the tree: an entry of a folder read, at a depth.
type row struct {
	path  string
	depth int
	entry proto.FileEntry
}

// layout lays the tree out again, as a new slice: the list may still be
// drawing the old one this frame.
func (v *View) layout() {
	var rows []row
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		for _, e := range v.dirs[dir].Entries {
			p := path.Join(dir, e.Name)
			rows = append(rows, row{path: p, depth: depth, entry: e})
			if e.Dir && v.open[p] {
				walk(p, depth+1)
			}
		}
	}
	walk("", 0)
	v.rows = rows
}

// Show shows the file at p, by its path in the session's folder, the next
// time the view draws.
func (v *View) Show(p string) { v.show = p }

// openFile shows the file at p, reading it; "" goes back to the tree.
func (v *View) openFile(o Options, p string) {
	if p != "" && p != v.file {
		v.fileList.ScrollTo(0, ui.Start)
	}
	v.file, v.content, v.lines = p, nil, nil
	if p == "" || o.Client == nil {
		return
	}
	v.reading = true
	cl, id := o.Client, o.Session.ID
	go func() {
		f, err := cl.ReadFile(id, p)
		o.Update(func() {
			if v.file != p {
				return
			}
			v.reading = false
			if err != nil {
				v.err = err.Error()
				return
			}
			v.content = &f
			v.lines = splitLines(f.Text)
		})
	}()
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := []string{}
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
