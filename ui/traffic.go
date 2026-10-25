package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/trafficview"
)

// The side panel's Traffic tool shows the connected host's interceptor
// (ui/trafficview): one view per connection, fed by the daemon's events.

// traffic is the Traffic tool's view for the connection gen.
func (a *App) trafficOf() *trafficview.View {
	if a.trafficView == nil || a.trafficGen != a.gen {
		a.trafficView, a.trafficGen = trafficview.New(), a.gen
	}
	return a.trafficView
}

func (a *App) trafficTab(c *ui.Context, s proto.SessionInfo) {
	top := a.parentOf(s)
	v := a.trafficOf()
	var tabs []proto.SessionInfo
	if top.ID != "" {
		tabs = append([]proto.SessionInfo{top}, a.tabsOf(top.ID)...)
	}
	v.Draw(c, trafficview.Options{
		Host:     a.client,
		HostName: a.host.Name,
		Session:  s,
		Tabs:     tabs,
		Update:   a.onMain,
		CopyText: func(text string) { mygo.Clipboard.WriteText(text) },
		SendToAgent: func(text string) {
			a.typeIntoSession(top.ID, string(pasteStart)+text+string(pasteEnd))
		},
		SaveFile:        saveDownload,
		BrowserCaptured: a.capturesWeb(top.ID),
		SetBrowserCapture: func(on bool) {
			a.webCapture.Store(top.ID, on)
			// Forwards made before go through the proxy, or around it,
			// once made again.
			a.stopForwards(top.ID)
			for _, t := range a.tabsOf(top.ID) {
				a.stopForwards(t.ID)
			}
		},
		OpenTab: func(id string) {
			if t, ok := a.sessionByID(id); ok {
				a.selectSession(t)
			}
		},
	})
}

// saveDownload keeps data in the Downloads folder, named name (made unique),
// and returns where.
func saveDownload(name string, data []byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		dir = home
	}
	path := filepath.Join(dir, name)
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(name)
		path = filepath.Join(dir, name[:len(name)-len(ext)]+"-"+strconv.Itoa(i)+ext)
	}
	return path, os.WriteFile(path, data, 0o600)
}

// trafficEvent applies the interceptor's events to the tool's view.
func (a *App) trafficEvent(ev proto.Event) func() {
	switch ev.Name {
	case proto.EventTrafficExchange:
		var s proto.TrafficSummary
		if json.Unmarshal(ev.Data, &s) != nil {
			return nil
		}
		return func() { a.trafficOf().Apply(s) }
	case proto.EventTrafficPaused:
		var ps []proto.TrafficPaused
		if json.Unmarshal(ev.Data, &ps) != nil {
			return nil
		}
		return func() {
			v := a.trafficOf()
			before := v.PausedCount()
			v.SetPaused(ps)
			if len(ps) > before {
				a.notify("A request is paused at a breakpoint: Traffic shows it.")
			}
		}
	case proto.EventTrafficAttached:
		var ts []proto.TrafficTarget
		if json.Unmarshal(ev.Data, &ts) != nil {
			return nil
		}
		return func() { a.trafficOf().SetTargets(ts) }
	}
	return nil
}

// capturesWeb reports whether session's Browser tool previews go through
// the interceptor; on the main thread.
func (a *App) capturesWeb(session string) bool {
	on, _ := a.webCapture.Load(a.topOf(session))
	b, _ := on.(bool)
	return b
}

// topOf is the session a tab belongs to (itself for a session), from the
// session list: on the main thread.
func (a *App) topOf(id string) string {
	for _, s := range a.sessions {
		if s.ID == id && s.Parent != "" {
			return s.Parent
		}
	}
	return id
}
