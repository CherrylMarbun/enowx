package ui

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/hostconn"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/androidview"
)

// The side panel's Android tool (ui/androidview): the devices of the
// session's host and, when that is a server, of this machine too, whose
// daemon the app then connects to beside it.

// localDaemon is a connection to this machine's daemon while the app is
// connected to a server: for devices here. It is safe for use by several
// goroutines.
type localDaemon struct {
	mu      sync.Mutex
	c       *client.Client
	err     error
	dialing bool
}

// get returns this machine's daemon, connecting the first time.
func (l *localDaemon) get(opts hostconn.Options) (*client.Client, error) {
	l.mu.Lock()
	if l.c != nil {
		select {
		case <-l.c.Done():
			l.c = nil
		default:
			c := l.c
			l.mu.Unlock()
			return c, nil
		}
	}
	l.mu.Unlock()
	c, err := hostconn.Connect(LocalHost(), opts)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err != nil {
		l.err = err
		return nil, err
	}
	if l.c != nil {
		c.Close()
		return l.c, nil
	}
	l.c, l.err = c, nil
	return c, nil
}

// connected is the connection made, nil before.
func (l *localDaemon) connected() *client.Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.c
}

func (l *localDaemon) close() {
	l.mu.Lock()
	c := l.c
	l.c = nil
	l.mu.Unlock()
	if c != nil {
		c.Close()
	}
}

func (a *App) androidOf() *androidview.View {
	if a.androidView == nil || a.androidGen != a.gen {
		if a.androidView != nil {
			a.androidView.Close()
		}
		a.androidView, a.androidGen = androidview.New(), a.gen
	}
	return a.androidView
}

func (a *App) androidTab(c *ui.Context, s proto.SessionInfo) {
	v := a.androidOf()
	gen := a.gen
	opts := a.hostOptions(gen)
	hs := []androidview.Host{{ID: a.host.ID, Name: a.host.Name, Client: a.client, Session: true, Local: a.host.ID == hosts.LocalID}}
	if a.host.ID != hosts.LocalID {
		if lc := a.local.connected(); lc != nil {
			hs = append(hs, androidview.Host{ID: hosts.LocalID, Name: LocalHost().Name, Client: lc, Local: true})
		} else if !a.local.dialing {
			// This machine's devices join once its daemon answers.
			a.local.dialing = true
			go func() {
				a.local.get(opts)
				a.onMain(func() { a.local.dialing = false })
			}()
		}
	}
	v.Draw(c, androidview.Options{
		Hosts:   hs,
		Session: s,
		Update:  a.onMain,
		OpenTab: func(id string) {
			if t, ok := a.sessionByID(id); ok {
				a.selectSession(t)
			}
		},
		ForwardPort: func(port int) { a.forwardPort(s.ID, proto.Port{Port: port, Addr: "127.0.0.1"}) },
		Local: func() (*client.Client, error) {
			if a.host.ID == hosts.LocalID {
				return a.client, nil
			}
			return a.local.get(opts)
		},
		NewScreenView: a.newScreenView(),
	}, func() bool { return gen == a.gen && a.panelShows(panelAndroid) })
}

// newScreenView makes the web views device screens show in, or nil where
// this device has none.
func (a *App) newScreenView() func(url string, done func(androidview.ScreenView, error)) {
	win := a.win
	if win == nil || !webViewAvailable() {
		return nil
	}
	return func(url string, done func(androidview.ScreenView, error)) {
		go func() {
			wv, err := win.NewWebView(mygo.WebViewOptions{URL: url})
			a.onMain(func() {
				if err != nil {
					done(nil, err)
					return
				}
				done(webPage{wv}, nil)
			})
		}()
	}
}
