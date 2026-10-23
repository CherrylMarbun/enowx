package ui

import (
	"slices"

	"github.com/enowx/enowx/proto"
)

// layout is how a project's sessions are laid out: one at a time or side
// by side, in how many columns, and which ones the split view leaves out.
// It is remembered per host and project between launches.
type layout struct {
	Split   bool     `json:"split,omitempty"`
	Columns int      `json:"columns,omitempty"` // 0: as square a grid as fits
	Hidden  []string `json:"hidden,omitempty"`  // session IDs
}

// splitColumns are the column counts the split view offers; 0 is Auto.
var splitColumns = []int{0, 1, 2, 3, 4}

func (a *App) layoutKey() string { return a.host.ID + " " + a.project }

// layout is the current project's layout.
func (a *App) layout() layout { return a.state.Layouts[a.layoutKey()] }

// setLayout changes the current project's layout.
func (a *App) setLayout(change func(*layout)) {
	l := a.layout()
	l.Hidden = slices.Clone(l.Hidden)
	change(&l)
	if a.state.Layouts == nil {
		a.state.Layouts = map[string]layout{}
	}
	if !l.Split && l.Columns == 0 && len(l.Hidden) == 0 {
		delete(a.state.Layouts, a.layoutKey())
		return
	}
	a.state.Layouts[a.layoutKey()] = l
}

// splitOn reports whether the current project shows its sessions side by
// side.
func (a *App) splitOn() bool { return a.layout().Split }

func (a *App) setSplit(on bool) { a.setLayout(func(l *layout) { l.Split = on }) }

// splitSessions are the sessions the split view shows: all but those left
// out, or all when that would leave none.
func (a *App) splitSessions(sessions []proto.SessionInfo) []proto.SessionInfo {
	hidden := a.layout().Hidden
	shown := slices.DeleteFunc(slices.Clone(sessions), func(s proto.SessionInfo) bool { return slices.Contains(hidden, s.ID) })
	if len(shown) == 0 {
		return sessions
	}
	return shown
}

// hideFromSplit leaves session id out of the split view, or brings it back.
func (a *App) hideFromSplit(id string, hide bool) {
	a.setLayout(func(l *layout) {
		l.Hidden = slices.DeleteFunc(l.Hidden, func(h string) bool { return h == id })
		if hide {
			l.Hidden = append(l.Hidden, id)
		}
	})
}

// forgetInLayouts drops a removed session from every layout.
func (a *App) forgetInLayouts(id string) {
	for key, l := range a.state.Layouts {
		if i := slices.Index(l.Hidden, id); i >= 0 {
			l.Hidden = slices.Delete(slices.Clone(l.Hidden), i, i+1)
			a.state.Layouts[key] = l
		}
	}
}
