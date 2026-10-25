package canvasview

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/proto"
)

// A mockup's HTML shows in a web view of this device, at the frame's size
// zoomed to fit; the shapes above it are drawn natively. An agent's change
// to one element goes into the page as it is, so the page doesn't reload
// at each, and the element changed is outlined a moment.

// Page is a web view showing a mockup.
type Page interface {
	Show(c *ui.Context) ui.Element
	LoadHTML(html string)
	// Eval runs JavaScript in the page and returns its value; it waits,
	// so it runs off the main thread.
	Eval(code string) (any, error)
	SetZoom(f float64)
	Close()
}

// Web makes and keeps the web view mockups show in.
type Web struct {
	// NewPage makes a page and hands it to done on the main thread.
	NewPage func(done func(Page, error))
	// Asset reads a canvas's file, off the main thread.
	Asset func(canvas, name string) ([]byte, error)
	// Update runs a function on the main thread.
	Update func(func())

	page    Page
	making  bool
	shown   string // canvas/frame/version loaded
	html    string // the frame's HTML loaded, its shapes left out
	pending string // a document to load once the page is made
	zoom    float64
	sel     string // the element outlined as selected
}

// Close closes the page, as the page goes.
func (w *Web) Close() {
	if w.page != nil {
		w.page.Close()
		w.page = nil
	}
	w.shown, w.html = "", ""
}

func key(c proto.Canvas, f proto.CanvasFrame) string { return c.ID + "/" + f.ID }

// show loads frame f of c when it isn't the one loaded.
func (w *Web) show(c proto.Canvas, f proto.CanvasFrame, _ *proto.Canvas) {
	if w.shown == key(c, f) && w.html == f.HTML {
		return
	}
	w.load(c, f)
}

// load loads frame f's HTML whole, its files fetched first.
func (w *Web) load(c proto.Canvas, f proto.CanvasFrame) {
	w.shown, w.html = key(c, f), f.HTML
	names := canvas.AssetNames(f.HTML)
	bare := proto.CanvasFrame{Width: f.Width, Height: f.Height, Background: f.Background, HTML: f.HTML}
	if len(names) == 0 || w.Asset == nil {
		w.setDoc(canvas.FrameDocument(bare, nil))
		return
	}
	shown := w.shown
	go func() {
		files := map[string][]byte{}
		for _, n := range names {
			if data, err := w.Asset(c.ID, n); err == nil {
				files[n] = data
			}
		}
		doc := canvas.FrameDocument(bare, func(name string) ([]byte, error) {
			if data, ok := files[name]; ok {
				return data, nil
			}
			return nil, errNoAsset
		})
		w.Update(func() {
			if w.shown == shown {
				w.setDoc(doc)
			}
		})
	}()
}

var errNoAsset = errors.New("canvas: no such asset")

func (w *Web) setDoc(doc string) {
	if w.page == nil {
		w.pending = doc
		return
	}
	w.page.LoadHTML(doc)
}

// apply puts an agent's or the user's change into the page: one element
// in place, else the frame anew.
func (w *Web) apply(c proto.Canvas, f proto.CanvasFrame, _ *proto.Canvas, ch proto.CanvasChange) {
	if w.shown != key(c, f) || w.page == nil || ch.Frame != f.ID || ch.Element == "" {
		w.show(c, f, nil)
		return
	}
	if f.HTML == w.html {
		return
	}
	var code string
	switch ch.Op {
	case proto.CanvasOpHTMLRemove:
		code = "(function(){var e=document.getElementById(" + js(ch.Element) + ");if(e)e.remove();return true})()"
	case proto.CanvasOpHTMLPatch, proto.CanvasOpHTMLText:
		outer, err := canvas.ElementHTML(f.HTML, ch.Element)
		if err != nil || strings.Contains(outer, "assets/") {
			w.load(c, f)
			return
		}
		code = "(function(id,html,parent){var e=document.getElementById(id);" +
			"if(e){e.outerHTML=html}else{var p=parent?document.getElementById(parent):document.body;if(!p)return false;p.insertAdjacentHTML('beforeend',html)}" +
			"var n=document.getElementById(id);if(n){n.scrollIntoView({block:'nearest'});n.style.outline='2px solid #6366f1';n.style.outlineOffset='2px';" +
			"setTimeout(function(){n.style.outline='';n.style.outlineOffset=''},1600)}return true})(" +
			js(ch.Element) + "," + js(outer) + "," + js(canvas.ParentID(f.HTML, ch.Element)) + ")"
	default:
		w.load(c, f)
		return
	}
	w.html = f.HTML
	page, shown := w.page, w.shown
	go func() {
		v, err := page.Eval(code)
		if ok, _ := v.(bool); err != nil || !ok {
			w.Update(func() {
				if w.shown == shown {
					w.load(c, f)
				}
			})
		}
	}()
}

// outline marks element id as the one selected, "" none.
func (w *Web) outline(id string) {
	if w.page == nil || w.sel == id {
		return
	}
	w.sel = id
	page := w.page
	code := "(function(id){document.querySelectorAll('[data-enowx-sel]').forEach(function(e){e.removeAttribute('data-enowx-sel');e.style.boxShadow=''});" +
		"var e=id&&document.getElementById(id);if(e){e.setAttribute('data-enowx-sel','');e.style.boxShadow='0 0 0 2px #6366f1';e.scrollIntoView({block:'nearest'})}})(" + js(id) + ")"
	go func() { _, _ = page.Eval(code) }()
}

func js(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// view places the page, w×h at zoom, making it the first time.
func (w *Web) view(c *ui.Context, width, height float32, zoom float64) ui.Element {
	if w.page == nil {
		if !w.making && w.NewPage != nil {
			w.making = true
			w.NewPage(func(p Page, err error) {
				w.making = false
				if err != nil {
					return
				}
				w.page = p
				if w.pending != "" {
					p.LoadHTML(w.pending)
					w.pending = ""
				}
			})
		}
		return ui.Box(c).Size(width, height)
	}
	if zoom != w.zoom {
		w.zoom = zoom
		w.page.SetZoom(zoom)
	}
	return w.page.Show(c).Size(width, height)
}
