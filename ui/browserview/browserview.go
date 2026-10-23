// Package browserview is the side panel's Browser tool: what a session's
// dev server serves, in a web view of this device, with back, forward,
// reload, the address, a pick of the session's ports, DevTools, and a
// screenshot to hand the agent. On a server, the app forwards the port
// here first, so only HTTP and WebSocket traffic (hot reload) crosses the
// connection. See docs/plans/preview.md.
package browserview

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/widget"
)

// refreshEvery is how often a shown view asks for the session's ports.
const refreshEvery = 2 * time.Second

// Page is the web view a View shows. The app makes it, once per session,
// and the view only places it: one the view doesn't place keeps its page.
type Page interface {
	// Show places the page in the view, as ui.WebView does.
	Show(c *ui.Context) ui.Element
	Load(url string)
	Reload()
	Back()
	Forward()
	CanBack() bool
	CanForward() bool
	URL() string
	Loading() bool
	ToggleDevTools()
	// Capture is a PNG of the page.
	Capture() ([]byte, error)
	Close()
}

// Options connect a view to the app.
type Options struct {
	Client  *client.Client
	Session proto.SessionInfo
	// Update runs a function on the main thread; Wanted, called there,
	// reports whether the view still shows, else its poller stops.
	Update func(func())
	Wanted func() bool
	// Supported says this device has a web view; without, the view offers
	// the browser.
	Supported bool
	// NewPage makes a page loading url and hands it to done on the main
	// thread, or why it couldn't.
	NewPage func(url string, done func(Page, error))
	// Origin is where port p of the session's host shows here, as
	// "http://localhost:5173": forwarded first for a server.
	Origin func(p proto.Port) (string, error)
	// OpenURL opens a URL in the browser; Attach hands a screenshot (a
	// PNG) to the session's agent.
	OpenURL func(url string)
	Attach  func(png []byte)
	// Port and Path are where the view starts, as last left; Remember
	// keeps where it is.
	Port     int
	Path     string
	Remember func(port int, path string)
}

// View is a session's browser. It belongs to the main thread; its poller
// reports through Options.Update.
type View struct {
	ports   []proto.Port
	loaded  bool
	err     string
	polling bool
	poke    chan struct{}

	port    int    // the port shown, 0 before one is picked
	origin  string // where it shows here
	page    Page
	making  bool   // a page is on the way
	stopped bool   // the port closed; reload when it is back
	address string // the address field, being edited
	editing bool
	started bool // Options.Port and Path were taken
	width   int  // the page's width, as a device's; 0 fits the room
	sizes   widget.ContextMenu
}

// widths are the page widths offered, as devices': 0 fits the room.
var widths = []struct {
	width int
	name  string
}{{0, "Fit"}, {1280, "Laptop · 1280"}, {768, "Tablet · 768"}, {390, "Phone · 390"}}

// New returns a view with nothing open.
func New() *View { return &View{poke: make(chan struct{}, 1)} }

// Close closes the view's page: its session went.
func (v *View) Close() {
	if v.page != nil {
		v.page.Close()
		v.page = nil
	}
}

func (v *View) follow(o Options) {
	if v.polling || o.Client == nil {
		return
	}
	v.polling = true
	cl, id := o.Client, o.Session.ID
	go func() {
		for {
			got, err := cl.Ports(id)
			keep := make(chan bool, 1)
			o.Update(func() {
				if !o.Wanted() {
					v.polling = false
					keep <- false
					return
				}
				keep <- true
				v.loaded = true
				if err != nil {
					v.err = client.Explain(err)
					return
				}
				v.err = ""
				v.setPorts(o, got.Ports)
			})
			if !<-keep {
				return
			}
			select {
			case <-time.After(refreshEvery):
			case <-v.poke:
			}
		}
	}()
}

// setPorts takes the session's ports in: picks one when none is, and
// notices the one shown stop and come back.
func (v *View) setPorts(o Options, ports []proto.Port) {
	v.ports = ports
	open := v.port != 0 && slices.ContainsFunc(ports, func(p proto.Port) bool { return p.Port == v.port })
	switch {
	case v.port == 0 && len(ports) > 0:
		p, path := ports[0], "/"
		if !v.started {
			v.started = true
			if i := slices.IndexFunc(ports, func(p proto.Port) bool { return p.Port == o.Port }); i >= 0 {
				p, path = ports[i], o.Path
			}
		}
		v.show(o, p, path)
	case v.port != 0 && !open:
		v.stopped = true
	case v.stopped && open:
		v.stopped = false
		if v.page != nil {
			v.page.Reload()
		}
	}
}

// show shows port p at path.
func (v *View) show(o Options, p proto.Port, path string) {
	origin, err := o.Origin(p)
	if err != nil {
		v.err = err.Error()
		return
	}
	v.port, v.origin, v.stopped, v.err = p.Port, origin, false, ""
	target := origin + cleanPath(path)
	if v.page != nil {
		v.page.Load(target)
		return
	}
	if v.making || o.NewPage == nil || !o.Supported {
		return
	}
	v.making = true
	o.NewPage(target, func(page Page, err error) {
		v.making = false
		if err != nil {
			v.err = err.Error()
			return
		}
		v.page = page
	})
}

// path is the page's address without the origin: what the address field
// shows.
func (v *View) path() string {
	if v.page == nil {
		return ""
	}
	u := v.page.URL()
	if rest, ok := strings.CutPrefix(u, v.origin); ok {
		return cleanPath(rest)
	}
	return u
}

// navigate loads what the address field says: a path of the port shown, or a
// URL of the port's origin.
func (v *View) navigate(o Options) {
	text := strings.TrimSpace(v.address)
	if u, err := url.Parse(text); err == nil && u.Host != "" {
		text = u.RequestURI()
	}
	if v.page != nil {
		v.page.Load(v.origin + cleanPath(text))
	}
	if o.Remember != nil {
		o.Remember(v.port, cleanPath(text))
	}
}

func cleanPath(p string) string {
	if p == "" || p[0] != '/' {
		return "/" + p
	}
	return p
}

func portLabel(p int) string { return ":" + strconv.Itoa(p) }
