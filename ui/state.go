package ui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/theme"
)

// appState is what the app remembers between launches, in StateFile: the
// host it showed, and per host the project and session that were open.
type appState struct {
	Host            string             `json:"host,omitempty"`
	Projects        map[string]string  `json:"projects,omitempty"` // host ID → project path
	Sessions        map[string]string  `json:"sessions,omitempty"` // host ID → session ID
	SkipPermissions *bool              `json:"skipPermissions,omitempty"`
	Chats           map[string]bool    `json:"chats,omitempty"`       // session IDs shown as a chat
	NotesAgents     map[string]string  `json:"notesAgents,omitempty"` // note ID → the agent its Notes page talks to
	CloneDirs       map[string]string  `json:"cloneDirs,omitempty"`   // host ID → the folder cloned into last
	Theme           string             `json:"theme,omitempty"`       // palette ID
	Accent          string             `json:"accent,omitempty"`      // #RRGGBB replacing the palette's accent
	RailWide        bool               `json:"railWide,omitempty"`    // the rail shows labels
	Layouts         map[string]layout  `json:"layouts,omitempty"`     // "<host ID> <project path>" → layout
	Panel           sidePanel          `json:"panel,omitempty"`
	Updates         updatePrefs        `json:"updates,omitempty"`
	Editor          string             `json:"editor,omitempty"`       // the editor last opened (editor.id)
	Previews        map[string]preview `json:"previews,omitempty"`     // session ID → where its browser was
	BrowserViews    map[string]bool    `json:"browserViews,omitempty"` // session IDs showing the Browser as their view
	TrafficViews    map[string]bool    `json:"trafficViews,omitempty"` // session IDs showing Traffic as their view
	AutoPreview     map[string][]int   `json:"autoPreview,omitempty"`  // "<host ID> <project>" → ports previewed when opened
	QuietPorts      bool               `json:"quietPorts,omitempty"`   // no card when a session opens a port
	CloseQuits      bool               `json:"closeQuits,omitempty"`   // closing the window quits, rather than keeping the app in the tray
	// What New session starts with (Settings): an agent, the project
	// folder rather than a new branch, a time budget in minutes.
	DefaultAgent  string `json:"defaultAgent,omitempty"`
	ProjectFolder bool   `json:"projectFolder,omitempty"`
	DefaultBudget int    `json:"defaultBudget,omitempty"`
	NotifyOff     bool   `json:"notifyOff,omitempty"` // no desktop notifications
	// StopAgentsOnQuit stops this device's agents when the app quits; they
	// keep working without it otherwise.
	StopAgentsOnQuit bool `json:"stopAgentsOnQuit,omitempty"`
}

// palette is the palette the user picked, with their accent.
func (st appState) palette() theme.Palette {
	var accent ui.Color
	if st.Accent != "" {
		accent = ui.Hex(st.Accent)
	}
	return theme.Lookup(st.Theme, accent)
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
