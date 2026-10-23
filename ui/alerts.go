package ui

import (
	"slices"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/proto"
)

// alert shows a desktop notification when a session the user isn't
// looking at comes to need them: its agent asks something, or finished
// its turn; and when an agent reached its time budget, looked at or not.
// old is the session as it was before info.
func (a *App) alert(old, info proto.SessionInfo) {
	body := a.alertBody(old, info)
	if body == "" {
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

// alertBody is what a notification says of the change from old to info,
// "" for nothing worth one.
func (a *App) alertBody(old, info proto.SessionInfo) string {
	if info.OverBudget && !old.OverBudget {
		return a.agentName(info.Agent) + " reached its time budget and was interrupted"
	}
	if old.State == info.State || info.Exited || a.watching(info) {
		return ""
	}
	switch info.State {
	case proto.StateWaiting:
		return a.agentName(info.Agent) + " needs your input"
	case proto.StateDone:
		return a.agentName(info.Agent) + " finished its turn"
	}
	return ""
}
