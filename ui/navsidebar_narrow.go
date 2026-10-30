package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The sidebar folded (Cmd+B): still one column, of icons. enowX's mark
// unfolds it; then the host, the pages, the sessions as their agents'
// icons with their status, the other pages' menu and the account. Notes
// and Canvas draw their own lists while it is folded.

// navNarrow is the folded sidebar's width.
const navNarrow = 58

func (a *App) navSidebarNarrow(c *ui.Context) {
	widget.Bento(ui.Column(c.Key("nav")).Width(navNarrow).Shrink(0).FillHeight(), theme.Panel).
		Transition(widget.LayoutMotion).Children(func() {
		ui.Column(c).FillWidth().Padding(8, 0).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			mark := ui.ButtonBase(c.Key("mark")).Size(36, 36).Center().Radius(theme.RadiusControl+1).
				Gradient(theme.AccentHigh, theme.Accent, 180).TextColor(theme.OnAccent).InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.3)).
				Label("Expand sidebar").Tooltip("Expand sidebar (" + modKey + "B)").Cursor(ui.CursorPointer)
			mark.Children(func() { ui.Icon(c, icon.Logo).FontSize(28) })
			if widget.Pressable(mark).Clicked() {
				a.state.RailWide = true
			}
			a.narrowHost(c)
			navDivider(c)
			page := func(id, label string, svg *ui.SVG, enabled bool) {
				on := a.navPage() == id && (id != "" || a.page == "")
				b := widget.IconButton(c.Key("page:"+id), svg, label, on)
				if !enabled {
					b.Opacity(0.4)
				} else if b.Clicked() {
					a.page = id
				}
			}
			page("", "Sessions", icon.Terminal, true)
			page(pageNotes, "Notes", icon.Notebook, a.client != nil)
			page(pageCanvas, "Canvas", icon.Canvas, a.client != nil)
			navDivider(c)
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(4, 0).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, e := range a.projectEntries() {
					for _, p := range e.projects {
						a.narrowProject(c, p)
					}
				}
			})
		})
		ui.Column(c).FillWidth().Padding(6, 0, 8, 0).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			a.navMore(c)
			a.railAccount(c, false)
		})
	})
}

// narrowHost is the host as an icon with its connection's dot, the menu of
// hosts on a click.
func (a *App) narrowHost(c *ui.Context) {
	col, state := a.connectionState()
	b := ui.ButtonBase(c.Key("host-switch")).Size(40, 40).Radius(theme.RadiusControl).Center().Cursor(ui.CursorPointer).
		Label("Host: " + a.host.Name + ", " + state).Tooltip(a.host.Name + " · " + state)
	if b.Hovered() {
		b.Background(theme.Hover)
	}
	b.Children(func() {
		ui.Icon(c, hostIcon(a.host)).FontSize(widget.RailIconSize).TextColor(theme.TextSoft)
		ui.Box(c).Size(10, 10).Radius(5).Background(theme.Panel).Center().
			AttachTo(b, ui.AnchorBottomRight, ui.AnchorBottomRight).Margin(0, 4, 4, 0).Children(func() {
			widget.Dot(c, col, false, 6)
		})
	})
	a.menu.OpenBelow(b, "host-switch", 40)
	a.menu.Show(c, b, "host-switch", func(done func()) { a.hostMenuItems(c, done) })
}

func navDivider(c *ui.Context) { ui.Box(c).Size(26, 1).Margin(2, 0).Background(theme.Border) }
