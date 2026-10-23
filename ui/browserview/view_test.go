package browserview

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// fakePage records what the view asks of it.
type fakePage struct {
	url      string
	reloads  int
	devtools int
	closed   bool
}

func (p *fakePage) Show(c *ui.Context) ui.Element { return ui.Text(c, "page "+p.url) }
func (p *fakePage) Load(url string)               { p.url = url }
func (p *fakePage) Reload()                       { p.reloads++ }
func (p *fakePage) Back()                         {}
func (p *fakePage) Forward()                      {}
func (p *fakePage) CanBack() bool                 { return false }
func (p *fakePage) CanForward() bool              { return false }
func (p *fakePage) URL() string                   { return p.url }
func (p *fakePage) Loading() bool                 { return false }
func (p *fakePage) ToggleDevTools()               { p.devtools++ }
func (p *fakePage) Capture() ([]byte, error)      { return []byte("png"), nil }
func (p *fakePage) Close()                        { p.closed = true }

func options(page *fakePage, remembered *string) Options {
	return Options{
		Supported: true,
		NewPage: func(url string, done func(Page, error)) {
			page.url = url
			done(page, nil)
		},
		Origin:   func(p proto.Port) (string, error) { return "http://localhost:" + strconv.Itoa(p.Port), nil },
		OpenURL:  func(string) {},
		Attach:   func([]byte) {},
		Remember: func(port int, path string) { *remembered = strconv.Itoa(port) + path },
	}
}

func TestBrowserFollowsTheSessionsDevServer(t *testing.T) {
	page := &fakePage{}
	var remembered string
	o := options(page, &remembered)
	v := New()
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 600, 500)
	v.loaded = true
	tt.Frame()
	if !tt.HasText("Waiting for a dev server: start one in the session (npm run dev, say) and it shows here.") {
		t.Fatalf("before a port: %q", tt.Texts())
	}

	vite := proto.Port{Port: 5173, Process: "node"}
	api := proto.Port{Port: 8080, Process: "go"}
	v.setPorts(o, []proto.Port{vite, api})
	tt.Frame()
	if page.url != "http://localhost:5173/" || !tt.HasText("page http://localhost:5173/") {
		t.Fatalf("first port: url %q, %q", page.url, tt.Texts())
	}

	// Another port, picked.
	if err := tt.Click(":8080 go"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if page.url != "http://localhost:8080/" || remembered != "8080/" {
		t.Fatalf("picked 8080: url %q, remembered %q", page.url, remembered)
	}

	// The dev server restarts: the page reloads when it is back.
	v.setPorts(o, []proto.Port{vite})
	tt.Frame()
	if !tt.HasText("The dev server on :8080 stopped. The page comes back when it does.") {
		t.Fatalf("stopped: %q", tt.Texts())
	}
	v.setPorts(o, []proto.Port{vite, api})
	tt.Frame()
	if page.reloads != 1 {
		t.Fatalf("reloads = %d", page.reloads)
	}

	if err := tt.Click("DevTools"); err != nil {
		t.Fatal(err)
	}
	if page.devtools != 1 {
		t.Fatal("DevTools didn't open")
	}
	v.Close()
	if !page.closed {
		t.Fatal("the page outlived its session")
	}
}

func TestBrowserStartsWhereItWasLeft(t *testing.T) {
	page := &fakePage{}
	var remembered string
	o := options(page, &remembered)
	o.Port, o.Path = 3000, "/settings"
	v := New()
	v.setPorts(o, []proto.Port{{Port: 5173}, {Port: 3000}})
	if page.url != "http://localhost:3000/settings" {
		t.Fatalf("url = %q", page.url)
	}
}

func TestBrowserWithoutAWebViewOffersTheBrowser(t *testing.T) {
	o := options(&fakePage{}, new(string))
	o.Supported = false
	v := New()
	v.loaded = true
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 600, 400)
	tt.Frame()
	if !tt.HasText("This device has no web view to show pages in. Open them in the browser instead.") {
		t.Fatalf("%q", tt.Texts())
	}
}

func TestPageWidthsAsDevices(t *testing.T) {
	page := &fakePage{}
	o := options(page, new(string))
	v := New()
	v.loaded = true
	v.setPorts(o, []proto.Port{{Port: 5173}})
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 900, 500)
	tt.Frame()
	if err := tt.Click("Page width: Fit"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Phone · 390"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if v.width != 390 {
		t.Fatalf("width %d", v.width)
	}
	r, ok := tt.Find("page http://localhost:5173/")
	if !ok || r.W > 390 {
		t.Fatalf("the page is %+v wide", r)
	}
}
