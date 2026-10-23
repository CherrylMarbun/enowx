package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/fileview"
)

// The Files tab of the side panel (see package fileview).

// filesTab draws the Files tab for session s.
func (a *App) filesTab(c *ui.Context, s proto.SessionInfo) {
	if a.fileViews == nil {
		a.fileViews = map[string]*fileview.View{}
	}
	v := a.fileViews[s.ID]
	if v == nil {
		v = fileview.New()
		a.fileViews[s.ID] = v
	}
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
