package ui

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/checkview"
)

// The Checks tab of the side panel (see package checkview).

func (a *App) checkViewOf(id string) *checkview.View {
	if a.checkViews == nil {
		a.checkViews = map[string]*checkview.View{}
	}
	v := a.checkViews[id]
	if v == nil {
		v = checkview.New()
		a.checkViews[id] = v
	}
	return v
}

// checksTab draws the Checks tab for session s.
func (a *App) checksTab(c *ui.Context, s proto.SessionInfo) {
	v := a.checkViewOf(s.ID)
	gen := a.gen
	v.Draw(c, checkview.Options{
		Client:  a.client,
		Session: s,
		Update:  a.onMain,
		Wanted: func() bool {
			return gen == a.gen && a.panelShows(panelChecks) && a.selected == s.ID && a.checkViews[s.ID] == v
		},
	})
}

// checksEvent pokes the Checks view of the session whose checks changed.
func (a *App) checksEvent(data json.RawMessage) func() {
	var ref proto.SessionRef
	if json.Unmarshal(data, &ref) != nil {
		return nil
	}
	return func() {
		if v := a.checkViews[ref.ID]; v != nil {
			v.Poke()
		}
	}
}
