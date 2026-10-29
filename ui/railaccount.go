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
	accountPersonal = "space:"      // the user's own space
	accountSpace    = "space:team:" // a team's, its ID after
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
	spaces, team := a.spacesOf().list, a.space()
	where := "Personal"
	if team != "" {
		where = a.spaceName(team)
	}
	trig.Label("Account: " + st.Account).Tooltip(st.Account + " · enowX Cloud · " + where)
	trig.Children(func() {
		initial := "?"
		if st.Account != "" {
			initial = strings.ToUpper(string([]rune(st.Account)[0]))
		}
		face := ui.Box(c).Size(widget.RailIconSize+4, widget.RailIconSize+4).Radius((widget.RailIconSize + 4) / 2).Shrink(0).
			Background(theme.Accent).TextColor(theme.OnAccent).Center()
		face.Children(func() {
			ui.Text(c, initial).FontSize(11).FontWeight(700)
		})
		if team != "" {
			// The team whose space shows, on the face's corner.
			ui.Box(c).AttachTo(face, ui.AnchorBottomRight, ui.AnchorBottomRight).Margin(0, -4, -3, 0).Radius(4).Padding(1).Background(theme.Panel).Children(func() {
				widget.TeamMark(c, team, where, 11)
			})
		}
		if wide {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
				ui.Text(c, st.Account).FontSize(12.5).TextColor(theme.Text).SingleLine()
				sub := "@" + st.GitHub
				if len(spaces) > 0 || st.GitHub == "" {
					sub = where
				}
				ui.Text(c, sub).FontSize(11).TextColor(theme.Muted).SingleLine()
			})
			if len(spaces) > 0 {
				ui.Icon(c, icon.ChevronsUD).FontSize(11).TextColor(theme.Muted)
			}
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
		if len(spaces) > 0 {
			widget.Divider(c)
			widget.PopupLabel(c, "SPACE")
			space := func(id, team, name, sub string) {
				it := sel.Item(id)
				widget.PopupItem(it, it.Highlighted()).TextColor(theme.TextSoft).Children(func() {
					ui.Box(c).Size(28, 28).Center().Children(func() {
						if team == "" {
							ui.Icon(c, icon.Account).FontSize(14)
							return
						}
						widget.TeamMark(c, team, name, 18)
					})
					ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
						ui.Text(c, name).TextColor(theme.Text).SingleLine()
						ui.Text(c, sub).FontSize(11).TextColor(theme.Muted).SingleLine()
					})
					if team == a.space() {
						ui.Icon(c, icon.Check).FontSize(13).TextColor(theme.Accent)
					}
				})
			}
			space(accountPersonal, "", "Personal", "Only you, on your devices")
			for _, sp := range spaces {
				space(accountSpace+sp.Team, sp.Team, sp.Name, "Team")
			}
		}
		widget.Divider(c)
		item(accountSettings, icon.Settings, "Teams and devices…")
		item(accountSignOut, icon.Close, "Sign out")
	})
	switch {
	case choice == accountPersonal:
		a.showSpace("")
		return
	case strings.HasPrefix(choice, accountSpace):
		a.showSpace(strings.TrimPrefix(choice, accountSpace))
		return
	}
	switch choice {
	case accountSettings:
		a.page, a.settingsSection = pageSettings, sectionCloud
	case accountSignOut:
		a.cloudSignOut()
	}
}
