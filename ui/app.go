// Package ui is the enowX desktop interface, drawn with MyGo's native UI.
package ui

import (
	"encoding/json"
	"io"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/tailnet"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/activityview"
	"github.com/enowx/enowx/ui/appwindow"
	"github.com/enowx/enowx/ui/browserview"
	"github.com/enowx/enowx/ui/chatview"
	"github.com/enowx/enowx/ui/checkview"
	"github.com/enowx/enowx/ui/creditview"
	"github.com/enowx/enowx/ui/diffview"
	"github.com/enowx/enowx/ui/fileview"
	"github.com/enowx/enowx/ui/folderpicker"
	"github.com/enowx/enowx/ui/libraryview"
	"github.com/enowx/enowx/ui/portview"
	"github.com/enowx/enowx/ui/syncview"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/usageview"
	"github.com/enowx/enowx/ui/widget"
)

// App is the state of the main window. Its fields are read and written on
// the main thread only; other goroutines go through win.Update.
type App struct {
	win   *mygo.Window
	theme *ui.Theme
	// onMain runs a function on the main thread: the window's Update.
	onMain func(func())
	opts   Options

	// The connection. gen counts connections, so callbacks of a previous
	// one (events, attaches, progress) are ignored after a switch.
	client      *client.Client
	gen         int
	host        hosts.Host
	connecting  string // progress while connecting; "" once connected
	connErr     string // why the last connection failed
	connFail    error  // the same, to tell what kind of failure it was
	notice      string // a.status is news, not a problem, while it equals this
	trust       *trustPrompt
	runAs       *runAsPrompt
	menu        widget.ContextMenu
	passphrase  *passphrasePrompt
	signIn      *signInPrompt
	sync        *syncview.Dialog
	appWindows  map[string]*appwindow.Window // desktop apps shown in their windows, by session ID
	runCommands map[string]*commandList      // project commands, by session ID, for its Run menu
	commandForm *commandForm
	library     *libraryview.View // the Settings page's library, for libraryHost
	libraryHost string
	setup       *setupPage                // the Setup page, for the host it was opened on
	setupPolled time.Time                 // when the setup's status was last read
	drift       map[string]int            // steps a host lacks to be what this device is, by host ID
	watches     map[string]*hostWatch     // the hosts not shown, by ID
	chooseRunAs string                    // the host whose next connection asks which user to run as
	connectLink string                    // a link the server gave while signing in: a Tailscale SSH check
	openSession string                    // a session to show once the host connected lists it
	openProject string                    // a project to show once the host connected shows it
	copying     *copyForm                 // the Copy to another host dialog
	tabs        map[string]string         // the tab each session shows in front, by session ID
	installs    map[string]func()         // install sessions, by session ID: what to check once each ends
	chats       map[string]*chatview.Chat // by session ID
	diffs       map[string]*diffview.View // by session ID
	fileViews   map[string]*fileview.View // by session ID
	// fileTabs are the files open in the side panel, by session ID; docs
	// show them, by session ID and path.
	fileTabs      map[string][]string
	docs          map[string]*fileview.Doc
	portViews     map[string]*portview.View     // by session ID
	browsers      map[string]*browserview.View  // by session ID
	activityViews map[string]*activityview.View // by session ID
	checkViews    map[string]*checkview.View    // by session ID
	// forwards are the connected server's ports forwarded here, by port.
	forwards map[int]*forward
	// sshRefused: the server's SSH refused to forward a port; forwards go
	// through the daemon. Read off the main thread.
	sshRefused atomic.Bool
	// portOffers are ports sessions just opened, offered to open.
	portOffers []portOffer
	// conflicts are the conflict radar's last look, by project path.
	conflicts   map[string][]proto.Conflict
	budget      *budgetForm // setting a session's budget
	agentLimit  *agentsForm // setting the host's limit of agents at once
	usage       usageview.Meter
	credits     creditview.Tracker
	updates     updater
	hostList    []hosts.Host
	passwords   map[string]string // SSH passwords when there is no keychain
	peers       []tailnet.Peer    // the tailnet's devices, if Tailscale runs
	peersErr    string            // why there are none
	peersLoaded bool              // the tailnet was asked at least once
	peersAsked  bool
	tailUser    string // SSH user for tailnet devices

	agents     []proto.AgentInfo
	daemonInfo proto.DaemonInfo // the connected daemon, as it described itself
	projects   []proto.ProjectInfo
	sessions   []proto.SessionInfo
	panes      map[string]*pane
	// removed remembers killed sessions, so a late update can't revive them.
	removed map[string]bool

	project   string          // selected project path
	collapsed map[string]bool // projects whose sessions are hidden
	selected  string          // selected session ID
	maximized string          // session shown alone, if any
	page      string          // a rail view shown instead of the sessions, or ""
	filter    string          // the session list's filter
	// accentInput is the Appearance page's custom accent as typed.
	accentInput string
	status      string // last error, shown in the status bar

	skipPermissions bool               // New session's default
	deleting        *proto.SessionInfo // asking to confirm a delete
	removing        *proto.ProjectInfo // asking to confirm removing a project
	state           appState           // remembered between launches
	savedState      string
	restoredView    bool // the remembered project and session were reopened

	newSession     newSessionForm
	palette        palette
	addProject     folderpicker.Picker
	addHost        addHostForm
	hostsPageShown bool // the Hosts page asked the tailnet since it opened
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
		panes:           map[string]*pane{},
		removed:         map[string]bool{},
		collapsed:       map[string]bool{},
		state:           loadState(opts.StateFile),
		skipPermissions: true,
	}
	theme.Apply(a.state.palette())
	a.theme = theme.Window()
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
		MinWidth:        minWindowWidth,
		MinHeight:       minWindowHeight,
		BackgroundColor: theme.Hex(theme.Background),
		// The view draws the title bar: the command bar sits in it.
		TitleBarStyle:  mygo.TitleBarHidden,
		TitleBarHeight: titleBarHeight,
		Content:        ui.View(a.view),
	})
	a.onMain = a.win.Update
	a.win.OnClosed(func() {
		a.disconnect()
		a.stopWatches()
	})
	a.reloadHosts()
	go a.loadPeers()
	a.connect(a.opts.Start)
	a.startUpdates()
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
		apply = func() {
			i := slices.IndexFunc(a.sessions, func(s proto.SessionInfo) bool { return s.ID == info.ID })
			var old proto.SessionInfo
			if i >= 0 {
				old = a.sessions[i]
			}
			a.upsert(info)
			a.installEnded(info)
			if i >= 0 {
				a.alert(old, info)
			}
		}
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
	case proto.EventChecksChanged:
		if apply = a.checksEvent(ev.Data); apply == nil {
			return
		}
	case proto.EventPortsChanged:
		if apply = a.portsEvent(ev.Data); apply == nil {
			return
		}
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
	info, _ := c.Info() // an older daemon leaves what it doesn't know zero
	if info.Display {
		_ = c.SetDisplayScale(appwindow.ScreenScale())
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
		a.agents, a.daemonInfo = agents, info
		a.setProjects(projects)
		for _, info := range sessions {
			a.upsert(info)
		}
		if id := a.openSession; id != "" {
			a.openSession = ""
			if s, ok := a.sessionByID(id); ok {
				a.selectSession(s) // picked from another host's list
			}
		}
		a.restoreView()
	})
}

func (a *App) setProjects(list []proto.ProjectInfo) {
	a.projects = list
	if p := a.openProject; p != "" && slices.ContainsFunc(list, func(i proto.ProjectInfo) bool { return i.Path == p }) {
		a.project, a.openProject = p, "" // just copied here
	}
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
		// A program that starts again (continued, or a queued agent's)
		// runs in a new terminal of the daemon: the old stream shows the
		// last one, so attach anew.
		if old := a.sessions[i]; (old.Exited || old.Queued) && !info.Exited && !info.Queued {
			if p := a.panes[info.ID]; p != nil {
				p.close()
			}
			delete(a.panes, info.ID)
		}
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
	delete(a.tabs, id)
	for parent, tab := range a.tabs {
		if tab == id {
			delete(a.tabs, parent)
		}
	}
	delete(a.chats, id)
	delete(a.state.Chats, id)
	delete(a.diffs, id)
	delete(a.fileViews, id)
	for _, p := range a.fileTabs[id] {
		delete(a.docs, id+"\x00"+p)
	}
	delete(a.fileTabs, id)
	delete(a.portViews, id)
	a.stopForwards(id)
	a.closeAppWindows(id)
	if b := a.browsers[id]; b != nil {
		b.Close()
		delete(a.browsers, id)
	}
	delete(a.state.Previews, id)
	delete(a.state.BrowserViews, id)
	delete(a.activityViews, id)
	delete(a.checkViews, id)
	a.forgetInLayouts(id)
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
		var conn io.ReadWriteCloser = st
		if c.Remote() {
			// Files dropped on it go to the server first.
			conn = &uploadConn{
				Stream: st,
				upload: func(path string) (string, error) { return c.Upload(id, path) },
				report: func(msg string, failed bool) {
					a.onMain(func() {
						if failed {
							a.status = msg
						} else {
							a.notify(msg)
						}
					})
				},
			}
		}
		t, err := terminal.New(terminal.Options{
			Conn:         conn,
			ResizeByConn: true,
			Theme:        theme.Terminal(),
			DarkTheme:    theme.Terminal(),
			Padding:      terminal.Padding{X: 10, Y: 8},
			OnClear: func(terminal.Clear) bool {
				go st.Clear()
				return true
			},
			OpenLink: func(link string) bool { return a.openLink(id, link) },
			// An image or files pasted reach the program as their paths
			// on its host, as agents take images.
			OnPaste: func(d transfer.Data) bool {
				s, ok := a.sessionByID(id)
				return ok && a.pasteFiles(s, d, func(paths string) {
					if term != nil {
						term.Paste(paths)
					}
				})
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

func (a *App) selectSession(s proto.SessionInfo) {
	if s.Parent != "" {
		// A tab: its session, with it in front.
		a.showTab(s.Parent, s.ID)
		s = a.parentOf(s)
	}
	a.selected = s.ID
	a.page = ""
	if a.showsChat(s.ID) {
		a.chatOf(s.ID).FocusComposer()
	}
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

// openFolder opens the folder picker on the connected host.
func (a *App) openFolder() {
	if a.client == nil {
		return
	}
	a.addProject.Open(a.client, a.win.Update)
}

// addProjectDialog is the folder picker; the folder picked becomes a
// project.
func (a *App) addProjectDialog(c *ui.Context) {
	a.addProject.View(c, a.host.Name, func(path string) {
		if cl := a.client; cl != nil {
			go a.addProjectPath(cl, path)
		}
	})
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
