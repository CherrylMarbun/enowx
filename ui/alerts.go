package ui

import (
	"slices"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/proto"
)

// alert shows a desktop notification when a session the user isn't
// looking at comes to need them: its agent asks something, or finished
// its turn. old is the session as it was before info.
func (a *App) alert(old, info proto.SessionInfo) {
	if old.State == info.State || info.Exited || a.watching(info) {
		return
	}
	var body string
	switch info.State {
	case proto.StateWaiting:
		body = a.agentName(info.Agent) + " needs your input"
	case proto.StateDone:
		body = a.agentName(info.Agent) + " finished its turn"
	default:
		return
	}
	// One notification per session: a newer one replaces it.
	n := mygo.NewNotification(mygo.NotificationOptions{
		ID: "session:" + info.ID, Title: a.sessionName(info), Subtitle: a.host.Name, Body: body,
	})
	gen, id := a.gen, info.ID
	n.OnClick(func() {
		a.win.Update(func() {
			if gen != a.gen {
				return
			}
			if i := slices.IndexFunc(a.sessions, func(s proto.SessionInfo) bool { return s.ID == id }); i >= 0 {
				a.selectSession(a.sessions[i])
			}
		})
		a.win.Focus()
	})
	go func() { _ = n.Show() }() // Show waits for the system; denied is fine
}

// watching reports whether the user sees session s now: the window has the
// focus and shows it, alone or in the split view of its project.
func (a *App) watching(s proto.SessionInfo) bool {
	if !a.win.IsFocused() || a.page != "" {
		return false
	}
	if a.splitOn() && a.maximized == "" && s.Project == a.project {
		return true
	}
	return a.selected == s.ID
}
