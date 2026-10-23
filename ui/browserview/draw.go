package browserview

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Draw shows the toolbar, then the page or why there is none.
func (v *View) Draw(c *ui.Context, o Options) {
	v.follow(o)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		v.toolbar(c, o)
		if v.err != "" {
			ui.Row(c).FillWidth().Padding(8, 14).Background(theme.Fade(theme.Error, 0.1)).Children(func() {
				ui.Text(c, v.err).FontSize(12).TextColor(theme.Error).Shrink(1)
			})
		}
		switch {
		case !o.Supported:
			v.note(c, o, "This device has no web view to show pages in. Open them in the browser instead.")
		case !v.loaded:
			v.note(c, o, "Looking for the session's dev server…")
		case v.port == 0:
			v.note(c, o, "Waiting for a dev server: start one in the session (npm run dev, say) and it shows here.")
		case v.stopped:
			v.note(c, o, "The dev server on "+portLabel(v.port)+" stopped. The page comes back when it does.")
		case v.page == nil:
			v.note(c, o, "Opening "+portLabel(v.port)+"…")
		default:
			v.page.Show(c).Grow(1).FillWidth().AlignSelf(ui.Stretch).Background(theme.Background)
		}
	})
}

func (v *View) note(c *ui.Context, o Options, text string) {
	ui.Column(c).Grow(1).FillWidth().Center().Padding(24).Gap(12).Children(func() {
		ui.Text(c, text).FontSize(12.5).TextColor(theme.Muted).TextAlign(ui.Center)
		if !o.Supported && v.port != 0 && widget.Button(c, widget.Secondary, icon.External, "Open in browser", "").Clicked() {
			o.OpenURL(v.origin + "/")
		}
	})
}

// toolbar is back, forward, reload, the address, the port, and what to do
// with the page.
func (v *View) toolbar(c *ui.Context, o Options) {
	ui.Column(c).Shrink(0).FillWidth().Padding(6, 10, 6, 10).Gap(6).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		ui.Row(c).FillWidth().Height(30).Gap(4).Children(func() {
			page := v.page
			if navButton(c, icon.ArrowLeft, "Back", page != nil && page.CanBack()) {
				page.Back()
			}
			if navButton(c, icon.ArrowRight, "Forward", page != nil && page.CanForward()) {
				page.Forward()
			}
			if navButton(c, icon.Sync, "Reload", page != nil) {
				page.Reload()
			}
			v.addressField(c, o)
			if page != nil && page.Loading() {
				widget.Spinner(c, 12)
			}
			if navButton(c, icon.Camera, "Screenshot to the agent", page != nil && o.Attach != nil) {
				if png, err := page.Capture(); err != nil {
					v.err = "Couldn't take a screenshot: " + err.Error()
				} else {
					o.Attach(png)
				}
			}
			if navButton(c, icon.Code, "DevTools", page != nil) {
				page.ToggleDevTools()
			}
			if navButton(c, icon.External, "Open in browser", v.port != 0) {
				o.OpenURL(v.origin + v.path())
			}
		})
		if len(v.ports) > 1 {
			v.portPicker(c, o)
		}
	})
}

// navButton is a toolbar button, dimmed when it can't act; it reports a
// click when it can.
func navButton(c *ui.Context, svg *ui.SVG, label string, can bool) bool {
	b := widget.IconButton(c.Key(label), svg, label, false)
	if !can {
		b.Opacity(0.35)
		return false
	}
	return b.Clicked()
}

// addressField shows the page's path, which the user edits to go
// elsewhere on the port.
func (v *View) addressField(c *ui.Context, o Options) {
	if !v.editing {
		v.address = v.path()
	}
	ui.Row(c).Grow(1).MinWidth(0).Height(28).PaddingX(10).Gap(4).Radius(theme.RadiusControl).
		Background(theme.Sunken).Border(1, theme.BorderStrong).Children(func() {
		if v.port != 0 {
			ui.Text(c, "localhost"+portLabel(v.port)).Font(theme.MonoFont).FontSize(11.5).TextColor(theme.Muted).SingleLine().Shrink(0)
		}
		in := ui.TextInputBase(c, &v.address).Grow(1).MinWidth(0).Font(theme.MonoFont).FontSize(11.5).Label("Address").Placeholder("/")
		v.editing = in.Focused()
		if in.Submitted() {
			v.navigate(o)
			v.editing = false
		}
	})
}

// portPicker switches among the session's ports.
func (v *View) portPicker(c *ui.Context, o Options) {
	labels := make([]string, len(v.ports))
	cur := -1
	for i, p := range v.ports {
		labels[i] = portLabel(p.Port)
		if p.Process != "" {
			labels[i] += " " + p.Process
		}
		if p.Port == v.port {
			cur = i
		}
	}
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, strconv.Itoa(len(v.ports))+" ports").FontSize(11).TextColor(theme.Muted).SingleLine()
		if i := widget.Segmented(c.Key("ports"), labels, cur, nil); i >= 0 {
			v.pick(o, v.ports[i])
		}
	})
}

// pick shows port p from its start.
func (v *View) pick(o Options, p proto.Port) {
	v.show(o, p, "/")
	if o.Remember != nil {
		o.Remember(p.Port, "/")
	}
}

// Show shows port p of the session, as the port offer and the Ports tool
// ask; it waits for the session's ports when they aren't read yet.
func (v *View) Show(o Options, p proto.Port) {
	v.started = true
	if !slices.ContainsFunc(v.ports, func(q proto.Port) bool { return q.Port == p.Port }) {
		v.ports = append(v.ports, p)
	}
	v.pick(o, p)
}
