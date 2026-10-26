package ui

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/textfmt"
)

// floatingSessions are the active sessions of source, in the order the pie
// shows them: what is waiting first, then what is working, then the rest,
// newest first within each. Exited ones are left out — a dead session is not
// active — but floatingSourceSessions still counts them, so an empty ring can
// say whether the source had nothing or everything there ended.
func (a *App) floatingSessions(source int) []proto.SessionInfo {
	return floatingActiveSessions(a.floatingSourceSessions(source))
}

// floatingSourceSessions are every session of source, exited ones included.
func (a *App) floatingSourceSessions(source int) []proto.SessionInfo {
	var all []proto.SessionInfo
	if source == floatingThisPC {
		switch {
		case a.host.ID == hosts.LocalID || a.host.ID == "":
			// Nothing connected is nothing remote either: This PC reads
			// what the app shows, else what the local watch knows.
			all = a.sessions
			if a.watches[hosts.LocalID] != nil && len(all) == 0 {
				all = a.watches[hosts.LocalID].status.sessions
			}
		case a.watches[hosts.LocalID] != nil:
			all = a.watches[hosts.LocalID].status.sessions
		}
	} else {
		// VPS is a server, never one of the user's own computers: the host
		// shown serves it only when it is one, and every server's sessions
		// are its own.
		switch {
		case a.host.ID != "" && a.host.ID != hosts.LocalID && !a.host.IsDevice():
			all = a.sessions
		default:
			for _, h := range a.allHosts() {
				if h.IsDevice() || h.ID == a.host.ID {
					continue
				}
				if w := a.watches[h.ID]; w != nil {
					all = append(all, w.status.sessions...)
				}
			}
		}
	}
	return all
}

// floatingActiveSessions are the sessions of all that can still be gone to
// — queued and hibernated ones too — in the pie's order.
func floatingActiveSessions(all []proto.SessionInfo) []proto.SessionInfo {
	out := make([]proto.SessionInfo, 0, len(all))
	for _, s := range all {
		if !s.Exited {
			out = append(out, s)
		}
	}
	slices.SortStableFunc(out, func(x, y proto.SessionInfo) int {
		if r := floatingRank(x) - floatingRank(y); r != 0 {
			return r
		}
		return y.Created.Compare(x.Created) // newest first
	})
	return out
}

// floatingRank orders the sessions: the one that needs the user first, then
// the one at work, then the rest.
func floatingRank(s proto.SessionInfo) int {
	switch {
	case s.State == proto.StateWaiting:
		return 0
	case isWorking(s):
		return 1
	}
	return 2
}

// floatingSessionWedges are the pie's wedges for list, one a slice, each
// carrying its session's status.
func (a *App) floatingSessionWedges(list []proto.SessionInfo) []floatingAction {
	acts := make([]floatingAction, 0, len(list))
	for _, s := range list {
		st := statusOf(s)
		identity := a.floatingSessionIdentity(s)
		// The pie names a session for a person glancing at it, not a
		// shell path: sessionName is the sidebar's, which shows the
		// terminal title as it is.
		name := a.floatingSessionName(identity)
		acts = append(acts, floatingAction{
			key:     "session:" + s.ID,
			label:   name,
			short:   name,
			svg:     floatingSessionIcon(identity),
			enabled: true,
			status:  &st,
			run:     func() { a.openFloatingSession(s) },
		})
	}
	return acts
}

// floatingSessionName is what the pie calls a session: the user's name for
// it, else its title when that says something, else its agent, else the word
// Session. A path is never a name: a plain shell reports its cwd or its
// program as the terminal title, and "C:\WINDOWS\System32\WindowsPowerShell\
// v1.0\powershell.exe" in a 56 DIP circle is unreadable.
func (a *App) floatingSessionName(s proto.SessionInfo) string {
	if n := strings.TrimSpace(s.Name); n != "" {
		return n
	}
	if t := cleanTitle(s.Title); t != "" && !floatingPathLike(t) && !a.genericTitle(s, t) {
		return t
	}
	if p := strings.TrimSpace(s.Prompt); p != "" {
		if line := textfmt.FirstLine(p, floatingNameBudget); line != "" && !floatingPathLike(line) {
			return line
		}
	}
	if s.Agent != "" {
		return a.agentName(s.Agent)
	}
	return "Session"
}

// floatingPathLike reports whether s reads as a file path rather than a name:
// a drive, a separator, or a program's extension. The pie falls back rather
// than showing one.
func floatingPathLike(s string) bool {
	if strings.ContainsAny(s, `/\`) {
		return true
	}
	switch strings.ToLower(filepath.Ext(s)) {
	case ".exe", ".com", ".bat", ".cmd", ".ps1", ".sh":
		return true
	}
	return false
}

// floatingClip shortens s to at most n runes, ending it with an ellipsis:
// what the hub draws, where the text API's own truncation would cut at a
// pixel and leave a path's tail, which says nothing.
func floatingClip(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// openFloatingSession goes to a session picked from the pie, connecting to the
// host it runs on first when the app is not already there.
//
// A session in the ring can belong to a server the app is not showing: the
// VPS source lists every server's sessions, and the pie is over whatever app
// the user is in. selectSession alone only marks it selected, and the
// workspace — which lists the shown host's sessions — has nothing to draw, so
// the panel folded away and the window came forward on a session that was not
// there. When the session's host is another, this connects to it and leaves the
// id in openSession, which refresh honours once the host lists it (the same
// path a session picked from the tray's cross-host list takes).
func (a *App) openFloatingSession(s proto.SessionInfo) {
	if h, ok := a.floatingSessionHost(s); ok && h.ID != a.host.ID {
		a.openSession = s.ID
		a.connect(h)
		return
	}
	a.selectSession(s)
}

// floatingSessionHost is the host s runs on, as the pie's sources see it: the
// shown host when it lists s, else the first watched host that does. ok is
// false when no host claims it, which is a session already gone.
func (a *App) floatingSessionHost(s proto.SessionInfo) (hosts.Host, bool) {
	if a.client != nil && slices.ContainsFunc(a.sessions, func(i proto.SessionInfo) bool { return i.ID == s.ID }) {
		return a.host, true
	}
	for _, h := range a.allHosts() {
		if w := a.watches[h.ID]; w != nil {
			if slices.ContainsFunc(w.status.sessions, func(i proto.SessionInfo) bool { return i.ID == s.ID }) {
				return h, true
			}
		}
	}
	return hosts.Host{}, false
}
