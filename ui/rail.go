package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Widths of the rail: icons only, or icons with labels.
const (
	// The bento border takes a DIP on each side, inside the width.
	railNarrow = widget.RailInset*2 + widget.RailItemSize + 2
	railWide   = 208
)

// rail is the column at the window's edge. Devices come first, then the
// servers, then the views that span a host; it folds down to icons.
func (a *App) rail(c *ui.Context) {
	wide := a.state.RailWide
	// Both widths keep the entries' left edge and lay them out from the
	// left, so folding only takes room away on the right.
	width := float32(railNarrow)
	if wide {
		width = railWide
	}
	col := widget.Bento(ui.Column(c.Key("rail")).Shrink(0).FillHeight().Gap(4), theme.Panel).
		Width(width).Padding(8, widget.RailInset).AlignItems(ui.Start).Transition(widget.LayoutMotion)
	col.Children(func() {
		a.railLogo(c, wide)
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Gap(4).AlignItems(ui.Start).Children(func() {
				a.railHosts(c, wide)
				a.railTeamHosts(c, wide)
			})
		})
		a.railPages(c, wide)
	})
}

// railHosts lists the devices, then the servers, then the button that
// adds one.
func (a *App) railHosts(c *ui.Context, wide bool) {
	var devices, servers []hosts.Host
	for _, h := range a.allHosts() {
		if h.IsDevice() {
			devices = append(devices, h)
		} else {
			servers = append(servers, h)
		}
	}
	widget.RailSection(c, "DEVICES", wide)
	for _, h := range devices {
		a.railHost(c.Key(h.ID), h, wide)
	}
	widget.RailSection(c, "VPS", wide)
	for _, h := range servers {
		a.railHost(c.Key(h.ID), h, wide)
	}
	if wide && len(servers) == 0 {
		ui.Text(c, "No servers yet").FontSize(11.5).TextColor(theme.Muted).Padding(2, 10).SingleLine()
	}
	a.hostMenu(c, wide)
}

// railLogo heads the rail: enowX's mark, and while wide its name and the
// button that folds the rail. Narrow, the mark unfolds it.
func (a *App) railLogo(c *ui.Context, wide bool) {
	ui.Row(c.Key("logo")).Height(widget.RailItemSize).FillWidth().Shrink(0).Gap(10).Margin(0, 0, 4, 0).Children(func() {
		mark := ui.ButtonBase(c.Key("mark")).Size(widget.RailItemSize, widget.RailItemSize).Shrink(0).Center().
			Radius(theme.RadiusControl+1).Gradient(theme.AccentHigh, theme.Accent, 180).TextColor(theme.OnAccent).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.3)).Label("enowX")
		if !wide {
			mark.Tooltip("Expand sidebar (" + modKey + "B)").Cursor(ui.CursorPointer)
		}
		mark.Children(func() { ui.Icon(c, icon.Logo).FontSize(28) })
		if widget.Pressable(mark).Clicked() && !wide {
			a.state.RailWide = true
		}
		if !wide {
			return
		}
		ui.Row(c.Key("name")).Gap(0).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, "ENOW").FontSize(13).FontWeight(700).LetterSpacing(2.4).TextColor(theme.TextStrong).SingleLine()
			ui.Text(c, "X").FontSize(13).FontWeight(700).LetterSpacing(2.4).TextColor(theme.Accent).SingleLine()
		}).Transition(ui.ElementTransition{Duration: 260 * time.Millisecond, Ease: widget.Out, Enter: &ui.Motion{X: -6, Opacity: 0}})
		if widget.IconButton(c, icon.Collapse, "Collapse sidebar ("+modKey+"B)", false).Clicked() {
			a.state.RailWide = false
		}
	})
}

// railPages are the views that span a host: the three used most as
// entries, the others as a row of icons while wide, then the account.
func (a *App) railPages(c *ui.Context, wide bool) {
	widget.RailSection(c, "WORKSPACE", wide)
	page := func(svg *ui.SVG, label, id string, enabled bool) bool {
		b := widget.RailItem(c.Key(id), svg, label, "rail-page", wide, a.page == id, ui.Color{})
		if !enabled {
			b.Opacity(0.4)
			return false
		}
		return b.Clicked()
	}
	if page(icon.Notebook, "Notes", pageNotes, a.client != nil) {
		a.togglePage(pageNotes)
	}
	if page(icon.Canvas, "Canvas", pageCanvas, a.client != nil) {
		a.togglePage(pageCanvas)
	}
	if page(icon.Key, "Credentials", pageVault, true) {
		a.openVault()
	}
	remote := a.client != nil && a.host.ID != hosts.LocalID
	syncLabel := "Sync environment"
	if !remote {
		syncLabel = "Sync environment: connect to a server first"
	}
	tools := []struct {
		svg     *ui.SVG
		label   string
		id      string
		enabled bool
		open    func()
	}{
		{icon.Settings, "Settings", pageSettings, a.client != nil, func() { a.togglePage(pageSettings) }},
		{icon.Palette, "Appearance", pageAppearance, true, func() { a.accentInput = a.state.Accent; a.togglePage(pageAppearance) }},
		{icon.Usage, "Resource usage", pageUsage, a.client != nil, func() { a.togglePage(pageUsage) }},
		{icon.Credits, "Credits", pageCredits, a.client != nil, func() { a.togglePage(pageCredits) }},
		{icon.Sync, syncLabel, pageSync, remote, a.openSync},
	}
	if !wide {
		for _, t := range tools {
			if page(t.svg, t.label, t.id, t.enabled) {
				t.open()
			}
		}
	} else {
		ui.Row(c.Key("tools")).FillWidth().Gap(2).Padding(2, 0).Children(func() {
			for _, t := range tools {
				b := widget.IconButton(c.Key("tool:"+t.id), t.svg, t.label, a.page == t.id)
				if !t.enabled {
					b.Opacity(0.4)
					continue
				}
				if b.Clicked() {
					t.open()
				}
			}
		})
	}
	ui.Box(c).Height(4)
	a.railAccount(c, wide)
}

// togglePage shows a rail view, or the sessions again if it is showing.
func (a *App) togglePage(id string) {
	if a.page == id {
		a.page = ""
	} else {
		a.page = id
	}
}

// railHost is one host of the rail; the current one carries the
// connection's state.
func (a *App) railHost(c *ui.Context, h hosts.Host, wide bool) {
	current := h.ID == a.host.ID
	status := ui.Color{}
	label := h.Name
	if !wide {
		label += " · " + hostDetail(h)
	}
	if current {
		col, state := a.connectionState()
		status = col
		if !wide {
			label = h.Name + " · " + state
		}
	} else if st, ok := a.statusOfHost(h); ok {
		// Another host: lit while it answers, amber while an agent there
		// waits for input.
		status = hostColor(st)
		if !wide {
			label = h.Name + " · " + hostSummary(st)
		}
	}
	if widget.RailItem(c, hostIcon(h), label, "rail-host", wide, current, status).Tooltip(h.Name+" · "+hostDetail(h)).Clicked() && !current {
		a.connect(h)
	}
}
