package browserview

import (
	"context"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/unslopview"
)

// Unslop in the Browser: the toolbar's button has the session's agent
// redesign the page's code where it looks AI-made, for a page of the
// project running on localhost; the panel shows the run over the page,
// and the page each read's marks, drawn again as the page reloads.

// localURL says u is on this device: localhost or a loopback address.
func localURL(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	h := strings.ToLower(p.Hostname())
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// unslopButton starts an unslop of the page shown.
func (v *View) unslopButton(c *ui.Context, o Options, page Page) {
	if o.Unslop == nil {
		return
	}
	label := "Unslop: find what makes this page look AI-made, and have the session's agent fix it in the code, pass by pass"
	can := page != nil && !o.Unslopping && localURL(page.URL())
	if page != nil && !localURL(page.URL()) {
		label = "Unslop works on the project's pages running here (localhost)"
	}
	if navButton(c, icon.Wand, label, can) {
		o.Unslop()
	}
}

// unslopOverlay draws o.UnslopRun's panel over the page, and its latest
// read's marks in the page.
func (v *View) unslopOverlay(c *ui.Context, o Options) {
	run := o.UnslopRun
	if run == nil || v.page == nil {
		v.scanned = ""
		return
	}
	// A page done loading (the dev server reloaded it) lost its marks.
	if loading := v.page.Loading(); loading != v.wasLoading {
		if v.wasLoading = loading; !loading {
			v.scanned = ""
		}
	}
	if key := strconv.Itoa(run.Scans) + "/" + v.page.URL(); !run.Done && v.scanned != key && !v.page.Loading() {
		v.scanned = key
		if ev, ok := v.page.(Evaluator); ok {
			code := unslopview.ScanScript + "(" + unslopview.ScanItems(run) + ")"
			go func() {
				time.Sleep(300 * time.Millisecond)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, _ = ev.Eval(ctx, code)
			}()
		}
	}
	if run.Done || o.Unslopping {
		c.After(time.Second)
	}
	v.panel.Draw(c, run, unslopview.PanelOptions{Reading: "Reading the page…", AgentName: o.AgentName})
}
