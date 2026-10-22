package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// appState is what the app remembers between launches, in StateFile: the
// host it showed, and per host the project and session that were open.
type appState struct {
	Host            string            `json:"host,omitempty"`
	Projects        map[string]string `json:"projects,omitempty"` // host ID → project path
	Sessions        map[string]string `json:"sessions,omitempty"` // host ID → session ID
	SkipPermissions *bool             `json:"skipPermissions,omitempty"`
}

func loadState(path string) appState {
	var st appState
	if path == "" {
		return st
	}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &st)
	}
	return st
}

// saveState writes the state when it changed. It runs on the main thread
// at the end of each frame, so every way of switching host, project or
// session is remembered without each one saying so.
func (a *App) saveState() {
	if a.opts.StateFile == "" || a.host.ID == "" {
		return
	}
	st := a.state
	st.Host = a.host.ID
	if st.Projects == nil {
		st.Projects, st.Sessions = map[string]string{}, map[string]string{}
	}
	if a.client != nil && a.restoredView {
		st.Projects[a.host.ID] = a.project
		st.Sessions[a.host.ID] = a.selected
	}
	skip := a.skipPermissions
	st.SkipPermissions = &skip
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil || string(data) == a.savedState {
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.opts.StateFile), 0o700); err != nil {
		return
	}
	tmp := a.opts.StateFile + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil && os.Rename(tmp, a.opts.StateFile) == nil {
		a.state, a.savedState = st, string(data)
	}
}

// restoreView reopens the project and session that were open on this host,
// once its state has loaded.
func (a *App) restoreView() {
	if a.restoredView {
		return
	}
	a.restoredView = true
	if p := a.state.Projects[a.host.ID]; p != "" {
		for _, proj := range a.projects {
			if proj.Path == p {
				a.project = p
			}
		}
	}
	if id := a.state.Sessions[a.host.ID]; id != "" {
		for _, s := range a.sessions {
			if s.ID == id {
				a.selectSession(s)
			}
		}
	}
}
