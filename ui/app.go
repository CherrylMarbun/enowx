// Package ui is the enowX desktop interface, drawn with MyGo's native UI.
package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// App is the state of the main window. Its fields are read and written on
// the main thread only; other goroutines go through win.Update. shown is
// the exception: the client's read goroutine checks it to know whether the
// window exists yet.
type App struct {
	win    *mygo.Window
	shown  atomic.Pointer[mygo.Window]
	client *client.Client
	host   string

	sessions []proto.SessionInfo
	panes    map[string]*pane
	// removed remembers killed sessions, so a late update can't revive them.
	removed  map[string]bool
	selected string
	command  string
	status   string
}

// pane shows one attached session.
type pane struct {
	term   *terminal.Terminal
	stream *client.Stream
}

// New returns the app. host labels the machine the daemon runs on. Pass
// HandleEvent to client.New, then hand the client to SetClient.
func New(host string) *App {
	return &App{host: host, panes: map[string]*pane{}, removed: map[string]bool{}}
}

// SetClient sets the daemon connection. Call it before Open.
func (a *App) SetClient(c *client.Client) { a.client = c }

// Open shows the main window and attaches to every session. Call it from
// mygo.App.WhenReady.
func (a *App) Open() *mygo.Window {
	a.win = mygo.NewWindow(mygo.WindowOptions{
		Title:   "enowX",
		Width:   1280,
		Height:  800,
		Content: ui.View(a.view),
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
		a.win.Update(func() { a.status = "Disconnected from daemon" })
	}()
	return a.win
}

// HandleEvent applies a daemon event. It runs on the client's read
// goroutine and hands the change to the main thread.
func (a *App) HandleEvent(ev proto.Event) {
	// Before the window opens, Open's refresh picks up the sessions.
	win := a.shown.Load()
	if win == nil {
		return
	}
	switch ev.Name {
	case proto.EventSessionUpdated:
		var info proto.SessionInfo
		if json.Unmarshal(ev.Data, &info) != nil {
			return
		}
		win.Update(func() { a.upsert(info) })
	case proto.EventSessionRemoved:
		var ref proto.SessionRef
		if json.Unmarshal(ev.Data, &ref) != nil {
			return
		}
		win.Update(func() { a.remove(ref.ID) })
	}
}

func (a *App) refresh() {
	list, err := a.client.Sessions()
	a.win.Update(func() {
		if err != nil {
			a.status = err.Error()
			return
		}
		for _, info := range list {
			a.upsert(info)
		}
	})
}

// upsert records a session and attaches to it once.
func (a *App) upsert(info proto.SessionInfo) {
	if a.removed[info.ID] {
		return
	}
	found := false
	for i := range a.sessions {
		if a.sessions[i].ID == info.ID {
			a.sessions[i] = info
			found = true
		}
	}
	if !found {
		a.sessions = append(a.sessions, info)
		if a.selected == "" {
			a.selected = info.ID
		}
	}
	if _, ok := a.panes[info.ID]; !ok {
		a.panes[info.ID] = nil // attaching
		go a.attach(info.ID)
	}
}

func (a *App) remove(id string) {
	a.removed[id] = true
	for i := range a.sessions {
		if a.sessions[i].ID == id {
			a.sessions = append(a.sessions[:i], a.sessions[i+1:]...)
			break
		}
	}
	if p := a.panes[id]; p != nil {
		p.close()
	}
	delete(a.panes, id)
	if a.selected == id {
		a.selected = ""
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
		term.Resize(cols, rows)
	}
	a.win.Update(func() {
		if _, wanted := a.panes[id]; !wanted {
			st.Close()
			close(ready)
			return
		}
		t, err := terminal.New(terminal.Options{
			Conn:         st,
			ResizeByConn: true,
			OnClear: func(terminal.Clear) bool {
				go st.Clear()
				return true
			},
		})
		if err != nil {
			st.Close()
			close(ready)
			delete(a.panes, id)
			a.status = err.Error()
			return
		}
		term = t
		close(ready)
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

func (a *App) createSession() {
	cmd := strings.Fields(a.command)
	go func() {
		_, err := a.client.CreateSession(proto.SessionCreateParams{Command: cmd})
		a.win.Update(func() {
			if err != nil {
				a.status = err.Error()
			} else {
				a.status = ""
			}
		})
	}()
}

func (a *App) killSelected() {
	id := a.selected
	if id == "" {
		return
	}
	go func() {
		if err := a.client.KillSession(id); err != nil {
			a.win.Update(func() { a.status = err.Error() })
		}
	}()
}

func (a *App) view(c *ui.Context) {
	ui.Row(c).Fill().Children(func() {
		a.sidebar(c)
		ui.Divider(c)
		a.grid(c)
	})
}

func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(260).FillHeight().Padding(12).Gap(10).Children(func() {
		ui.Text(c, "enowX").FontSize(18).Bold()
		ui.Text(c, a.host).TextColor(t.TextMuted)

		ui.Sidebar(c, &a.selected, func() {
			ui.SidebarSection(c, "Sessions", nil, func() {
				for _, s := range a.sessions {
					ui.SidebarItem(c.Key(s.ID), s.ID, nil, sessionLabel(s)).Children(func() {
						if s.Exited {
							ui.Badge(c, fmt.Sprintf("exit %d", s.ExitCode))
						}
					})
				}
			})
		}).Grow(1)

		ui.TextInput(c, &a.command).Placeholder("Command (empty = shell)").OnSubmit(a.createSession)
		ui.Row(c).Gap(8).Children(func() {
			if ui.PrimaryButton(c, "New session").Clicked() {
				a.createSession()
			}
			if ui.Button(c, "Kill").Clicked() {
				a.killSelected()
			}
		})
		if a.status != "" {
			ui.Text(c, a.status).TextColor(t.Danger)
		}
	})
}

func (a *App) grid(c *ui.Context) {
	var shown []proto.SessionInfo
	for _, s := range a.sessions {
		if a.panes[s.ID] != nil {
			shown = append(shown, s)
		}
	}
	if len(shown) == 0 {
		ui.Column(c).Grow(1).FillHeight().Center().Children(func() {
			ui.Text(c, "No sessions yet. Start one from the sidebar.").TextColor(c.Theme().TextMuted)
		})
		return
	}
	cols := int(math.Ceil(math.Sqrt(float64(len(shown)))))
	rows := (len(shown) + cols - 1) / cols
	tracks := make([]ui.Track, rows)
	for i := range tracks {
		tracks[i] = ui.Fr(1)
	}
	ui.Grid(c).Grow(1).FillHeight().Columns(cols).RowTracks(tracks...).Gap(1).Background(c.Theme().Border).Children(func() {
		for _, s := range shown {
			p := a.panes[s.ID]
			ui.Column(c).Key(s.ID).Background(c.Theme().Background).Children(func() {
				ui.Row(c).PaddingX(8).PaddingY(4).Gap(6).Children(func() {
					ui.Text(c, sessionLabel(s)).FontSize(12).TextColor(c.Theme().TextMuted)
				})
				terminal.View(c.Key(s.ID), p.term).Grow(1).Padding(4)
			})
		}
	})
}

func sessionLabel(s proto.SessionInfo) string {
	if s.Title != "" {
		return s.Title
	}
	if len(s.Command) > 0 {
		return strings.Join(s.Command, " ")
	}
	return "shell"
}
