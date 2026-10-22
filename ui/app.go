// Package ui is the enowX desktop interface, drawn with MyGo's native UI.
package ui

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/tailnet"
	"github.com/enowx/enowx/proto"
)

// App is the state of the main window. Its fields are read and written on
// the main thread only; other goroutines go through win.Update.
type App struct {
	win   *mygo.Window
	theme *ui.Theme
	opts  Options

	// The connection. gen counts connections, so callbacks of a previous
	// one (events, attaches, progress) are ignored after a switch.
	client      *client.Client
	gen         int
	host        hosts.Host
	connecting  string // progress while connecting; "" once connected
	connErr     string // why the last connection failed
	trust       *trustPrompt
	runAs       *runAsPrompt
	menu        *contextMenu
	hostList    []hosts.Host
	passwords   map[string]string // SSH passwords when there is no keychain
	peers       []tailnet.Peer    // the tailnet's devices, if Tailscale runs
	peersErr    string            // why there are none
	peersLoaded bool              // the tailnet was asked at least once
	peersAsked  bool
	tailUser    string // SSH user for tailnet devices

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

	skipPermissions bool               // New session's default
	deleting        *proto.SessionInfo // asking to confirm a delete
	state           appState           // remembered between launches
	savedState      string
	restoredView    bool // the remembered project and session were reopened

	newSession  newSessionForm
	addProject  addProjectForm
	addHost     addHostForm
	manageHosts bool
}

// Options configures the app.
type Options struct {
	// Hosts is the device's list of servers.
	Hosts *hosts.Store
	// Start is the host to connect to first.
	Start hosts.Host
	// DaemonCommand starts this machine's daemon.
	DaemonCommand []string
	// OpenProject, if set, is opened as a project once connected.
	OpenProject string
	// StateFile keeps what the app reopens at the next launch.
	StateFile string
	// ResumeHost starts on the host shown when the app last quit, if it is
	// still saved, instead of Start.
	ResumeHost bool
}

// pane shows one attached session.
type pane struct {
	term   *terminal.Terminal
	stream *client.Stream
}

// LocalHost is this machine as a host.
func LocalHost() hosts.Host {
	name := "This PC"
	if runtime.GOOS == "darwin" {
		name = "This Mac"
	}
	return hosts.Host{ID: hosts.LocalID, Name: name}
}

// New returns the app.
func New(opts Options) *App {
	a := &App{
		opts:            opts,
		theme:           graphiteTheme(),
		panes:           map[string]*pane{},
		removed:         map[string]bool{},
		collapsed:       map[string]bool{},
		state:           loadState(opts.StateFile),
		skipPermissions: true,
	}
	if a.state.SkipPermissions != nil {
		a.skipPermissions = *a.state.SkipPermissions
	}
	if opts.ResumeHost && opts.Hosts != nil && a.state.Host != "" && a.state.Host != hosts.LocalID {
		if h, ok := opts.Hosts.Get(a.state.Host); ok {
			a.opts.Start = h
		}
	}
	return a
}

// Open shows the main window and connects to the start host. Call it from
// mygo.App.WhenReady.
func (a *App) Open() *mygo.Window {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "enowX",
		Width:           1280,
		Height:          800,
		BackgroundColor: "#0E0F12",
		Content:         ui.View(a.view),
	})
	a.win.OnClosed(a.disconnect)
	a.reloadHosts()
	go a.loadPeers()
	a.connect(a.opts.Start)
	return a.win
}

// handleEvent applies a daemon event of connection gen. It runs on the
// client's read goroutine and hands the change to the main thread.
func (a *App) handleEvent(gen int, ev proto.Event) {
	var apply func()
	switch ev.Name {
	case proto.EventSessionUpdated:
		var info proto.SessionInfo
		if json.Unmarshal(ev.Data, &info) != nil {
			return
		}
		apply = func() { a.upsert(info) }
	case proto.EventSessionRemoved:
		var ref proto.SessionRef
		if json.Unmarshal(ev.Data, &ref) != nil {
			return
		}
		apply = func() { a.remove(ref.ID) }
	case proto.EventProjectsChanged:
		var list []proto.ProjectInfo
		if json.Unmarshal(ev.Data, &list) != nil {
			return
		}
		apply = func() { a.setProjects(list) }
	default:
		return
	}
	a.win.Update(func() {
		if gen == a.gen {
			apply()
		}
	})
}

// refresh loads the daemon's state; it runs off the main thread.
func (a *App) refresh(c *client.Client, gen int, openProject string) {
	if openProject != "" {
		if _, err := c.AddProject(openProject); err != nil {
			a.win.Update(func() { a.status = err.Error() })
		}
	}
	agents, errA := c.Agents()
	projects, errP := c.Projects()
	sessions, errS := c.Sessions()
	a.win.Update(func() {
		if gen != a.gen {
			return
		}
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
		a.restoreView()
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
	if _, ok := a.panes[info.ID]; !ok && a.client != nil {
		a.panes[info.ID] = nil // attaching
		go a.attach(a.client, a.gen, info.ID)
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
func (a *App) attach(c *client.Client, gen int, id string) {
	st, err := c.Attach(id)
	if err != nil {
		a.win.Update(func() {
			if gen != a.gen {
				return
			}
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
		if _, wanted := a.panes[id]; !wanted || gen != a.gen {
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

// askDelete asks before deleting a session.
func (a *App) askDelete(s proto.SessionInfo) { a.deleting = &s }

func (a *App) killSession(id string) {
	c := a.client
	if c == nil {
		return
	}
	go func() {
		if err := c.KillSession(id); err != nil {
			a.win.Update(func() { a.status = err.Error() })
		}
	}()
}

// openFolder asks for a folder and adds it as a project: with the system's
// dialog on this machine, or by path on a remote host.
func (a *App) openFolder() {
	c := a.client
	if c == nil {
		return
	}
	if a.host.ID != hosts.LocalID {
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
		a.addProjectPath(c, paths[0])
	}()
}

// addProjectPath runs off the main thread.
func (a *App) addProjectPath(c *client.Client, path string) {
	info, err := c.AddProject(path)
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
	case s.Exited && s.Restored && s.ExitCode == -1:
		return sessionStatus{label: "stopped", color: colIdle}
	case s.Exited && s.ExitCode == 0:
		return sessionStatus{label: "done", color: colDone}
	case s.Exited:
		return sessionStatus{label: "exited " + strconv.Itoa(s.ExitCode), color: colError}
	case s.Active:
		return sessionStatus{label: "working", color: colWorking}
	}
	return sessionStatus{label: "idle", color: colIdle, hollow: true}
}

// sessionName labels a session: the name the user gave it, else what the
// program says it is doing (its terminal title), else its first prompt,
// else its branch or agent.
func (a *App) sessionName(s proto.SessionInfo) string {
	if s.Name != "" {
		return s.Name
	}
	if t := cleanTitle(s.Title); t != "" && !a.genericTitle(s, t) {
		return t
	}
	if s.Prompt != "" {
		return firstLine(s.Prompt, 60)
	}
	if s.Worktree && s.Branch != "" {
		return s.Branch
	}
	if s.Agent == "" && len(s.Command) > 0 {
		return filepath.Base(s.Command[0])
	}
	return a.agentName(s.Agent)
}

// cleanTitle drops the spinners and marks agents put before their title.
func cleanTitle(t string) string {
	return strings.TrimSpace(strings.TrimLeftFunc(t, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}))
}

// genericTitle reports titles that say nothing about the task: the agent's
// own name, or a shell's "user@host: dir".
func (a *App) genericTitle(s proto.SessionInfo, t string) bool {
	lower := strings.ToLower(t)
	ag := a.agentInfo(s.Agent)
	if lower == strings.ToLower(ag.Name) || lower == strings.ToLower(ag.Command) || lower == s.Agent {
		return true
	}
	for _, generic := range []string{"claude code", "claude", "codex", "openai codex", "opencode", "gemini", "gemini cli", "kiro", "kiro cli", "kiro-cli"} {
		if lower == generic {
			return true
		}
	}
	if at, colon := strings.Index(t, "@"), strings.Index(t, ":"); at > 0 && colon > at {
		return true
	}
	return strings.HasPrefix(t, "~") || strings.HasPrefix(t, "/")
}

func firstLine(s string, n int) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
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
