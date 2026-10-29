package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The rail's foot: the account the connected host is signed in to the
// cloud as. Signed out it signs in, one click (the dialog, cloudsignin.go,
// opens the browser on the code); signed in, a menu of the account.

const (
	accountSettings = "settings"
	accountSignOut  = "signout"
)

func (a *App) railAccount(c *ui.Context, wide bool) {
	if a.client == nil {
		return
	}
	cs := a.cloudOf()
	if !cs.loaded {
		return
	}
	st := cs.st
	switch {
	case !st.SignedIn && st.Pending == nil:
		label := "Sign in"
		if !wide {
			label = "Sign in to enowX Cloud: your teams"
		}
		b := widget.RailItem(c.Key("account"), icon.Account, label, "rail-account", wide, false, ui.Color{}).
			Tooltip("Sign in to enowX Cloud with GitHub: your teams, their notes and hosts")
		if b.Clicked() {
			a.openCloudSignIn()
		}
		return
	}
	if st.Pending != nil {
		// Waiting for the code: the dialog shows it.
		b := widget.RailItem(c.Key("account"), icon.Account, "Approve "+st.Pending.UserCode+" in the browser", "rail-account", wide, a.cloudSigningIn, theme.Warning)
		if b.Clicked() {
			a.cloudSigningIn = true
		}
		return
	}
	choice := ""
	sel := ui.SelectBase(c, &choice)
	trig := widget.RailEntry(sel.Trigger, wide).Cursor(ui.CursorPointer)
	if sel.Open() || trig.Hovered() {
		trig.Background(theme.Hover)
	}
	trig.Label("Account: " + st.Account).Tooltip(st.Account + " · enowX Cloud")
	trig.Children(func() {
		initial := "?"
		if st.Account != "" {
			initial = strings.ToUpper(string([]rune(st.Account)[0]))
		}
		ui.Box(c).Size(widget.RailIconSize+4, widget.RailIconSize+4).Radius((widget.RailIconSize + 4) / 2).Shrink(0).
			Background(theme.Accent).TextColor(theme.OnAccent).Center().Children(func() {
			ui.Text(c, initial).FontSize(11).FontWeight(700)
		})
		if wide {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
				ui.Text(c, st.Account).FontSize(12.5).TextColor(theme.Text).SingleLine()
				if st.GitHub != "" {
					ui.Text(c, "@"+st.GitHub).FontSize(11).TextColor(theme.Muted).SingleLine()
				}
			})
		}
	})
	sel.Popup(func(panel ui.Element) {
		widget.Popup(panel.AttachTo(trig, ui.AnchorBottomRight, ui.AnchorBottomLeft).Margin(0, 0, 0, 10).Width(240))
		item := func(id string, svg *ui.SVG, label string) {
			it := sel.Item(id)
			widget.PopupItem(it, it.Highlighted()).TextColor(theme.TextSoft).Children(func() {
				ui.Box(c).Size(28, 28).Center().Children(func() { ui.Icon(c, svg).FontSize(14) })
				ui.Text(c, label)
			})
		}
		widget.PopupLabel(c, "ENOWX CLOUD")
		ui.Column(c).Gap(1).Padding(2, 9, 8, 9).MaxWidth(228).Children(func() {
			ui.Text(c, st.Account).FontSize(13).FontWeight(600).SingleLine()
			sub := a.host.Name + " as " + deviceKind(st.DeviceKind)
			if st.GitHub != "" {
				sub = "@" + st.GitHub + " · " + sub
			}
			ui.Text(c, sub).FontSize(11.5).TextColor(theme.Muted).SingleLine()
		})
		widget.Divider(c)
		item(accountSettings, icon.Settings, "Teams and devices…")
		item(accountSignOut, icon.Close, "Sign out")
	})
	switch choice {
	case accountSettings:
		a.page, a.settingsSection = pageSettings, sectionCloud
	case accountSignOut:
		a.cloudSignOut()
	}
}
