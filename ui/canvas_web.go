package ui

import (
	"errors"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/canvasview"
)

// Mockups show in web views of this device: the Canvas page's, and one
// made for a moment, behind the window's content, to render a frame for
// an agent (canvas screenshot) or an export.

// canvasPage is a web view as canvasview uses it.
type canvasPage struct{ wv *mygo.WebView }

func (p canvasPage) Show(c *ui.Context) ui.Element { return ui.WebView(c, p.wv) }
func (p canvasPage) LoadHTML(html string)          { p.wv.Page().LoadHTML(html, "") }
func (p canvasPage) Eval(code string) (any, error) { return p.wv.Page().Eval(code) }
func (p canvasPage) SetZoom(f float64)             { p.wv.Page().SetZoomFactor(f) }
func (p canvasPage) Close()                        { p.wv.Destroy() }

// canvasWeb makes the Canvas page's web view of mockups; nil without one.
func (a *App) canvasWeb() *canvasview.Web {
	if !webViewAvailable() || a.win == nil {
		return nil
	}
	return &canvasview.Web{
		NewPage: func(done func(canvasview.Page, error)) {
			win := a.win
			go func() {
				wv, err := win.NewWebView(mygo.WebViewOptions{})
				a.onMain(func() {
					if err != nil {
						done(nil, err)
						return
					}
					done(canvasPage{wv}, nil)
				})
			}()
		},
		Asset: func(id, name string) ([]byte, error) {
			cl := a.client
			if cl == nil {
				return nil, errors.New("not connected")
			}
			return cl.CanvasAsset(id, name)
		},
		Update: a.onMain,
	}
}

// canvasRender is a frame being rendered: its web view, placed behind the
// window's content at the frame's size. The main thread owns it.
type canvasRender struct {
	wv   *mygo.WebView
	w, h float32
}

// renders runs one render at a time.
var renders sync.Mutex

// renderCanvas renders req's page and replies to the daemon.
func (a *App) renderCanvas(req proto.CanvasRenderRequest) {
	cl, win := a.client, a.win
	if cl == nil {
		return
	}
	wider := 0
	reply := func(data []byte, err error) {
		out := proto.CanvasRenderReply{ID: req.ID, Data: data, Wider: wider}
		if err != nil {
			out.Error = err.Error()
		}
		_ = cl.CanvasRenderReply(out)
	}
	if !webViewAvailable() || win == nil {
		go reply(nil, errors.New("this device has no web view to render HTML"))
		return
	}
	go func() {
		renders.Lock()
		defer renders.Unlock()
		wv, err := win.NewWebView(mygo.WebViewOptions{BackgroundColor: "#ffffff"})
		if err != nil {
			reply(nil, err)
			return
		}
		defer a.onMain(func() {
			a.rendering = nil
			wv.Destroy()
		})
		loaded := make(chan struct{}, 1)
		off := wv.Page().OnDidFinishLoad(func() {
			select {
			case loaded <- struct{}{}:
			default:
			}
		})
		defer off()
		a.onMain(func() { a.rendering = &canvasRender{wv: wv, w: float32(req.Width), h: float32(req.Height)} })
		wv.Page().LoadHTML(req.HTML, "")
		select {
		case <-loaded:
		case <-time.After(20 * time.Second):
			reply(nil, errors.New("the page didn't load"))
			return
		}
		time.Sleep(400 * time.Millisecond) // fonts and images settle
		if v, err := wv.Page().Eval(canvasview.WiderScript); err == nil {
			if n, ok := v.(float64); ok && n > 0 {
				wider = int(n)
			}
		}
		if req.Format == proto.RenderPDF {
			size := mygo.PageSize{Width: float64(req.Width) / 96, Height: float64(req.Height) / 96}
			reply(wv.Page().PrintToPDF(mygo.PDFOptions{PageSize: size, Margins: &mygo.Margins{}, Background: true}))
			return
		}
		// The web view may not be laid out yet at the first try.
		png, err := wv.CapturePage()
		for try := 0; try < 3 && (err != nil || len(png) == 0); try++ {
			time.Sleep(500 * time.Millisecond)
			png, err = wv.CapturePage()
		}
		if err == nil && len(png) == 0 {
			err = errors.New("the web view gave no picture")
		}
		reply(png, err)
	}()
}

// renderingView places the web view rendering a frame: laid out at the
// frame's size, but faded out, as a web view shows over the window's
// content wherever it is placed.
func (a *App) renderingView(c *ui.Context) {
	r := a.rendering
	if r == nil {
		return
	}
	ui.WebView(c.Key("canvas-render"), r.wv).Absolute().Left(0).Top(0).Size(r.w, r.h).Opacity(0).PassThrough()
}
