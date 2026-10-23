package ui

import (
	"path"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/fileview"
)

// The Files tool of the side panel (see package fileview), and the files
// opened from it, each in a tab of the panel's top row. Tabs belong to
// their session, as the panel follows the selected one.

// fileTabPrefix starts the panel's Tab for a file: "file:<path>".
const fileTabPrefix = "file:"

func (a *App) fileViewOf(id string) *fileview.View {
	if a.fileViews == nil {
		a.fileViews = map[string]*fileview.View{}
	}
	v := a.fileViews[id]
	if v == nil {
		v = fileview.New()
		a.fileViews[id] = v
	}
	return v
}

// fileOptions connect session s's file views to the app; wanted tells
// whether one still shows.
func (a *App) fileOptions(s proto.SessionInfo, wanted func() bool) fileview.Options {
	ed := a.preferredEditor()
	return fileview.Options{
		Client:       a.client,
		Session:      s,
		Update:       a.onMain,
		Wanted:       wanted,
		EditorName:   ed.name,
		OpenInEditor: func(path string, line int) { a.openInEditor(ed, path, line) },
		OpenFile:     func(p string) { a.openFileTab(s, p) },
	}
}

// filesTab draws the Files tool for session s.
func (a *App) filesTab(c *ui.Context, s proto.SessionInfo) {
	v := a.fileViewOf(s.ID)
	gen := a.gen
	v.Draw(c, a.fileOptions(s, func() bool {
		return gen == a.gen && a.panelShows(panelFiles) && a.selected == s.ID && a.fileViews[s.ID] == v
	}))
}

// openFileTab shows the file at p of session s's folder in a tab, opening
// one for it unless it has one.
func (a *App) openFileTab(s proto.SessionInfo, p string) {
	if a.fileTabs == nil {
		a.fileTabs = map[string][]string{}
	}
	if !slices.Contains(a.fileTabs[s.ID], p) {
		a.fileTabs[s.ID] = append(a.fileTabs[s.ID], p)
	}
	a.openPanel(fileTabPrefix + p)
}

// showFile shows the file at p of session s's folder in its tab.
func (a *App) showFile(s proto.SessionInfo, p string) { a.openFileTab(s, p) }

// closeFileTab closes session id's tab of the file at p; the Files tool
// shows in its place when it was in front.
func (a *App) closeFileTab(id, p string) {
	a.fileTabs[id] = slices.DeleteFunc(a.fileTabs[id], func(t string) bool { return t == p })
	delete(a.docs, id+"\x00"+p)
	if a.selected == id && a.state.Panel.Tab == fileTabPrefix+p {
		a.state.Panel.Tab = panelFiles
	}
}

// fileTabOf is the file of panel tab tab for session id, if it has it.
func (a *App) fileTabOf(id, tab string) (string, bool) {
	p, ok := strings.CutPrefix(tab, fileTabPrefix)
	return p, ok && slices.Contains(a.fileTabs[id], p)
}

// docTab draws session s's tab of the file at p.
func (a *App) docTab(c *ui.Context, s proto.SessionInfo, p string) {
	if a.docs == nil {
		a.docs = map[string]*fileview.Doc{}
	}
	key := s.ID + "\x00" + p
	d := a.docs[key]
	if d == nil {
		d = fileview.NewDoc(p)
		a.docs[key] = d
	}
	gen := a.gen
	d.Draw(c, a.fileOptions(s, func() bool {
		return gen == a.gen && a.panelShows(fileTabPrefix+p) && a.selected == s.ID && a.docs[key] == d
	}))
}

// fileTabName is a tab's label: the file's name.
func fileTabName(p string) string { return path.Base(p) }
