// Package ui is the enowX desktop interface, drawn with MyGo's native UI.
package ui

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// Host describes the machine the daemon runs on.
type Host struct {
	Name   string // "This Mac", "vps@sg-1"
	Detail string // "Local", "SSH"
	Remote bool
}

// App is the state of the main window. Its fields are read and written on
// the main thread only; other goroutines go through win.Update. shown is
// the exception: the client's read goroutine checks it to know whether the
// window exists yet.
type App struct {
	win    *mygo.Window
	shown  atomic.Pointer[mygo.Window]
	client *client.Client
	host   Host
	theme  *ui.Theme

	agents   []proto.AgentInfo
	projects []proto.ProjectInfo
	sessions []proto.SessionInfo
	panes    map[string]*pane
	// removed remembers killed sessions, so a late update can't revive them.
	removed map[string]bool

	project   string          // selected project path
	collapsed map[string]bool // projects whose sessions are hidden
	selected  string          // selected session ID
	maximized string          // session shown alone, if any
	status    string          // last error, shown in the status bar

	newSession newSessionForm
	addProject addProjectForm
}

// pane shows one attached session.
type pane struct {
	term   *terminal.Terminal
	stream *client.Stream
}

// New returns the app. Pass HandleEvent to client.New, then hand the
// client to SetClient.
func New(host Host) *App {
	return &App{
		host:      host,
		theme:     graphiteTheme(),
		panes:     map[string]*pane{},
		removed:   map[string]bool{},
		collapsed: map[string]bool{},
	}
}

// SetClient sets the daemon connection. Call it before Open.
func (a *App) SetClient(c *client.Client) { a.client = c }

// Open shows the main window and loads the daemon's state. Call it from
// mygo.App.WhenReady.
func (a *App) Open() *mygo.Window {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "enowX",
		Width:           1280,
		Height:          800,
		BackgroundColor: "#0E0F12",
		Content:         ui.View(a.view),
	})
	a.win.OnClosed(func() {
		for _, p := range a.panes {
			p.close()
		}
		a.client.Close()
	})
	a.shown.Store(a.win)
	go a.refresh()
	go func() {
		<-a.client.Done()
		a.win.Update(func() { a.status = "Disconnected from the daemon" })
	}()
	return a.win
}

// HandleEvent applies a daemon event. It runs on the client's read
// goroutine and hands the change to the main thread.
func (a *App) HandleEvent(ev proto.Event) {
	// Before the window opens, Open's refresh picks up the state.
	win := a.shown.Load()
	if win == nil {
		return
	}
	switch ev.Name {
	case proto.EventSessionUpdated:
		var info proto.SessionInfo
		if json.Unmarshal(ev.Data, &info) == nil {
			win.Update(func() { a.upsert(info) })
		}
	case proto.EventSessionRemoved:
		var ref proto.SessionRef
		if json.Unmarshal(ev.Data, &ref) == nil {
			win.Update(func() { a.remove(ref.ID) })
		}
	case proto.EventProjectsChanged:
		var list []proto.ProjectInfo
		if json.Unmarshal(ev.Data, &list) == nil {
			win.Update(func() { a.setProjects(list) })
		}
	}
}

func (a *App) refresh() {
	agents, errA := a.client.Agents()
	projects, errP := a.client.Projects()
	sessions, errS := a.client.Sessions()
	a.win.Update(func() {
		for _, err := range []error{errA, errP, errS} {
			if err != nil {
				a.status = err.Error()
				return
			}
		}
		a.agents = agents
		a.setProjects(projects)
		for _, info := range sessions {
			a.upsert(info)
		}
	})
}

func (a *App) setProjects(list []proto.ProjectInfo) {
	a.projects = list
	if a.project == "" || !slices.ContainsFunc(list, func(p proto.ProjectInfo) bool { return p.Path == a.project }) {
		a.project = ""
		if len(list) > 0 {
			a.project = list[0].Path
		}
	}
}

// upsert records a session and attaches to it once.
func (a *App) upsert(info proto.SessionInfo) {
	if a.removed[info.ID] {
		return
	}
	if i := slices.IndexFunc(a.sessions, func(s proto.SessionInfo) bool { return s.ID == info.ID }); i >= 0 {
		a.sessions[i] = info
	} else {
		a.sessions = append(a.sessions, info)
	}
	if _, ok := a.panes[info.ID]; !ok {
		a.panes[info.ID] = nil // attaching
		go a.attach(info.ID)
	}
}

func (a *App) remove(id string) {
	a.removed[id] = true
	a.sessions = slices.DeleteFunc(a.sessions, func(s proto.SessionInfo) bool { return s.ID == id })
	if p := a.panes[id]; p != nil {
		p.close()
	}
	delete(a.panes, id)
	if a.selected == id {
		a.selected = ""
	}
	if a.maximized == id {
		a.maximized = ""
	}
}

// attach runs off the main thread: it waits on the daemon, then builds the
// pane's terminal on the main thread.
func (a *App) attach(id string) {
	st, err := a.client.Attach(id)
	if err != nil {
		a.win.Update(func() {
			delete(a.panes, id)
			if strings.Contains(err.Error(), "not found") {
				a.remove(id) // killed before we attached
				return
			}
			a.status = err.Error()
		})
		return
	}
	// The terminal reads the stream from its own goroutine as soon as it
	// exists; resizes wait until it is assigned.
	ready := make(chan struct{})
	var term *terminal.Terminal
	st.OnResize = func(cols, rows int) {
		<-ready
		if term != nil {
			term.Resize(cols, rows)
		}
	}
	a.win.Update(func() {
		defer close(ready)
		if _, wanted := a.panes[id]; !wanted {
			st.Close()
			return
		}
		t, err := terminal.New(terminal.Options{
			Conn:         st,
			ResizeByConn: true,
			Theme:        terminalTheme,
			DarkTheme:    terminalTheme,
			Padding:      terminal.Padding{X: 10, Y: 8},
			OnClear: func(terminal.Clear) bool {
				go st.Clear()
				return true
			},
		})
		if err != nil {
			st.Close()
			delete(a.panes, id)
			a.status = err.Error()
			return
		}
		term = t
		a.panes[id] = &pane{term: t, stream: st}
	})
}

func (p *pane) close() {
	if p == nil {
		return
	}
	p.term.Close()
	p.stream.Close()
}

// projectSessions returns the sessions of a project, oldest first.
func (a *App) projectSessions(path string) []proto.SessionInfo {
	var out []proto.SessionInfo
	for _, s := range a.sessions {
		if s.Project == path || (s.Project == "" && s.Dir == path) {
			out = append(out, s)
		}
	}
	return out
}

func (a *App) currentProject() (proto.ProjectInfo, bool) {
	for _, p := range a.projects {
		if p.Path == a.project {
			return p, true
		}
	}
	return proto.ProjectInfo{}, false
}

func (a *App) selectSession(s proto.SessionInfo) {
	a.selected = s.ID
	if s.Project != "" {
		a.project = s.Project
		delete(a.collapsed, s.Project)
	}
	if a.maximized != "" {
		a.maximized = s.ID
	}
}

func (a *App) killSession(id string) {
	go func() {
		if err := a.client.KillSession(id); err != nil {
			a.win.Update(func() { a.status = err.Error() })
		}
	}()
}

// openFolder asks for a folder and adds it as a project: with the system's
// dialog on this machine, or by path on a remote host.
func (a *App) openFolder() {
	if a.host.Remote {
		a.addProject.open = true
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            a.win,
			Title:             "Open a folder as a project",
			ButtonLabel:       "Open",
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.addProjectPath(paths[0])
	}()
}

// addProjectPath runs off the main thread.
func (a *App) addProjectPath(path string) {
	info, err := a.client.AddProject(path)
	a.win.Update(func() {
		if err != nil {
			a.status = err.Error()
			return
		}
		a.status = ""
		a.project = info.Path
		a.selected = ""
		delete(a.collapsed, info.Path)
	})
}

// sessionStatus is what the UI shows for a session's state.
type sessionStatus struct {
	label  string
	color  ui.Color
	hollow bool // drawn as a ring: nothing is happening
}

func statusOf(s proto.SessionInfo) sessionStatus {
	switch {
	case s.Exited && s.ExitCode == 0:
		return sessionStatus{label: "done", color: colDone}
	case s.Exited:
		return sessionStatus{label: "exited " + strconv.Itoa(s.ExitCode), color: colError}
	case s.Active:
		return sessionStatus{label: "working", color: colWorking}
	}
	return sessionStatus{label: "idle", color: colIdle, hollow: true}
}

func sessionName(s proto.SessionInfo) string {
	if s.Name != "" {
		return s.Name
	}
	if s.Title != "" {
		return s.Title
	}
	if len(s.Command) > 0 {
		return filepath.Base(s.Command[0])
	}
	return "Shell"
}

func (a *App) agentName(id string) string {
	for _, ag := range a.agents {
		if ag.ID == id {
			return ag.Name
		}
	}
	if id == "" {
		return "Shell"
	}
	return id
}
