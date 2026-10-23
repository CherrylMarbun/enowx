package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/ui/browserview"
)

// The Browser tool of the side panel (see package browserview): one web
// view per session, made the first time it shows a page and kept while
// the session lasts, so switching tools or sessions keeps the page.

// webViewAvailable reports, once, whether this device can show pages.
var webViewAvailable = sync.OnceValue(mygo.WebViewAvailable)

// webPage is a session's web view, as browserview uses it.
type webPage struct{ wv *mygo.WebView }

func (p webPage) Show(c *ui.Context) ui.Element { return ui.WebView(c, p.wv) }
func (p webPage) Load(url string)               { _ = p.wv.Page().LoadURL(url) }
func (p webPage) Reload()                       { p.wv.Page().Reload() }
func (p webPage) Back()                         { p.wv.Page().GoBack() }
func (p webPage) Forward()                      { p.wv.Page().GoForward() }
func (p webPage) CanBack() bool                 { return p.wv.Page().CanGoBack() }
func (p webPage) CanForward() bool              { return p.wv.Page().CanGoForward() }
func (p webPage) URL() string                   { return p.wv.Page().URL() }
func (p webPage) Loading() bool                 { return p.wv.Page().IsLoading() }
func (p webPage) ToggleDevTools()               { p.wv.Page().ToggleDevTools() }
func (p webPage) Capture() ([]byte, error)      { return p.wv.CapturePage() }
func (p webPage) Close()                        { p.wv.Destroy() }

func (a *App) browserOf(id string) *browserview.View {
	if a.browsers == nil {
		a.browsers = map[string]*browserview.View{}
	}
	v := a.browsers[id]
	if v == nil {
		v = browserview.New()
		a.browsers[id] = v
	}
	return v
}

// browserOptions connect session s's browser to the app.
func (a *App) browserOptions(s proto.SessionInfo) browserview.Options {
	gen := a.gen
	last := a.state.Previews[s.ID]
	return browserview.Options{
		Client:    a.client,
		Session:   s,
		Update:    a.onMain,
		Supported: webViewAvailable(),
		Wanted: func() bool {
			return gen == a.gen && a.panelShows(panelBrowser) && a.selected == s.ID && a.browsers[s.ID] != nil
		},
		NewPage: func(url string, done func(browserview.Page, error)) {
			win := a.win
			go func() {
				wv, err := win.NewWebView(mygo.WebViewOptions{URL: url, Page: mygo.PageOptions{DevTools: mygo.DevToolsEnabled}})
				a.onMain(func() {
					if err != nil {
						done(nil, err)
						return
					}
					done(webPage{wv}, nil)
				})
			}()
		},
		Origin: func(p proto.Port) (string, error) {
			if a.client == nil || !a.client.Remote() {
				return "http://localhost:" + strconv.Itoa(p.Port), nil
			}
			a.forwardPort(p)
			l, ok := a.forwards[p.Port]
			if !ok {
				return "", fmt.Errorf("couldn't forward :%d from %s", p.Port, a.host.Name)
			}
			return "http://" + l.Addr().String(), nil
		},
		OpenURL: func(url string) { go func() { _ = mygo.Shell.OpenExternal(url) }() },
		Attach:  func(png []byte) { a.attachScreenshot(s, png) },
		Port:    last.Port,
		Path:    last.Path,
		Remember: func(port int, path string) {
			if a.state.Previews == nil {
				a.state.Previews = map[string]preview{}
			}
			a.state.Previews[s.ID] = preview{Port: port, Path: path}
		},
	}
}

// preview is where a session's browser was, remembered between launches.
type preview struct {
	Port int    `json:"port"`
	Path string `json:"path,omitempty"`
}

// browserTab draws the Browser tool for session s.
func (a *App) browserTab(c *ui.Context, s proto.SessionInfo) {
	a.browserOf(s.ID).Draw(c, a.browserOptions(s))
}

// previewPort shows port p of session s in its browser.
func (a *App) previewPort(s proto.SessionInfo, p proto.Port) {
	a.selectSession(s)
	a.openPanel(panelBrowser)
	a.browserOf(s.ID).Show(a.browserOptions(s), p)
}

// attachScreenshot saves a page's screenshot and types its path into
// session s, uploading it first to a server's session, so its agent can
// look at it.
func (a *App) attachScreenshot(s proto.SessionInfo, png []byte) {
	dir, err := transport.Dir()
	if err != nil {
		a.status = err.Error()
		return
	}
	path := filepath.Join(dir, "screenshots", "preview-"+time.Now().Format("20060102-150405")+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		a.status = err.Error()
		return
	}
	if err := os.WriteFile(path, png, 0o600); err != nil {
		a.status = err.Error()
		return
	}
	cl := a.client
	if cl == nil {
		return
	}
	go func() {
		target := path
		if cl.Remote() {
			remote, err := cl.Upload(s.ID, path)
			if err != nil {
				a.onMain(func() { a.status = "Couldn't send the screenshot: " + err.Error() })
				return
			}
			target = remote
		}
		a.onMain(func() {
			a.typeIntoSession(s.ID, escapePath(target)+" ")
			a.notify("Screenshot attached to " + a.sessionName(s) + "'s prompt")
		})
	}()
}
