package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/fileview"
)

// The Files tab of the side panel (see package fileview).

// filesTab draws the Files tab for session s.
func (a *App) filesTab(c *ui.Context, s proto.SessionInfo) {
	v := a.fileViewOf(s.ID)
	gen := a.gen
	v.Draw(c, fileview.Options{
		Client:  a.client,
		Session: s,
		Update:  a.onMain,
		Wanted: func() bool {
			return gen == a.gen && a.panelShows(panelFiles) && a.selected == s.ID && a.fileViews[s.ID] == v
		},
	})
}

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

// showFile shows the file at p of session s's folder in the Files tab.
func (a *App) showFile(s proto.SessionInfo, p string) {
	a.openPanel(panelFiles)
	a.fileViewOf(s.ID).Show(p)
}
