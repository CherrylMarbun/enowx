package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Settings › Cloud's teams: the account's, one open with its members and
// an invite to make, a new one, and joining with an invite.

type cloudTeams struct {
	list    []api.Team
	open    string   // the team shown with its members
	team    api.Team // that team, with its members
	newName string
	join    string
	role    int // the invite's role, of inviteRoles
	invite  api.Invite
	copied  widget.CopyFeedback
}

var inviteRoles = []string{api.RoleMember, api.RoleViewer, api.RoleAdmin}

func teamPath(p, team string) string { return strings.Replace(p, "{team}", team, 1) }

func (a *App) cloudTeamsSection(c *ui.Context, cs *cloudSettings) {
	ts := &cs.teams
	widget.Field(c, "Teams", func() {
		if len(ts.list) == 0 {
			ui.Text(c, "No teams yet: make one, or join one with an invite.").FontSize(12).TextColor(theme.Muted)
		}
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			for _, t := range ts.list {
				a.cloudTeamRow(c, cs, t)
			}
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() { widget.Input(c, &ts.newName, "New team's name").FillWidth() })
			if widget.Button(c, widget.Secondary, icon.Plus, "Make team", "").Clicked() && strings.TrimSpace(ts.newName) != "" {
				name := strings.TrimSpace(ts.newName)
				a.cloudDo("Making "+name+"…", func(cl *client.Client) error {
					return cl.CloudCall("POST", api.PathTeams, api.TeamParams{Name: &name}, nil)
				}, func() { ts.newName = ""; a.loadCloud() })
			}
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() { widget.Input(c, &ts.join, "An invite link, to join its team").FillWidth() })
			if widget.Button(c, widget.Secondary, icon.Link, "Join", "").Clicked() && strings.TrimSpace(ts.join) != "" {
				link := strings.TrimSpace(ts.join)
				a.cloudDo("Joining…", func(cl *client.Client) error {
					return cl.CloudCall("POST", api.PathJoin, api.JoinParams{Token: link}, nil)
				}, func() { ts.join = ""; a.loadCloud() })
			}
		})
	})
}

// cloudTeamRow is one team; open, its members and an invite to make.
func (a *App) cloudTeamRow(c *ui.Context, cs *cloudSettings, t api.Team) {
	ts := &cs.teams
	open := ts.open == t.ID
	ui.Column(c.Key("team:"+t.ID)).FillWidth().Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border).Children(func() {
		head := ui.ButtonBase(c.Key("head:"+t.ID)).FillWidth().Padding(10, 12).Gap(10).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label("Team " + t.Name)
		if head.Hovered() {
			head.Background(theme.Hover)
		}
		head.Children(func() {
			chevron := icon.ChevronRight
			if open {
				chevron = icon.ChevronDown
			}
			ui.Icon(c, chevron).FontSize(11).TextColor(theme.Muted)
			ui.Text(c, t.Name).FontSize(13).FontWeight(600).Grow(1).Shrink(1).MinWidth(0).SingleLine()
			widget.Tag(c, t.Role, theme.TextSoft)
		})
		if head.Clicked() {
			if open {
				ts.open = ""
				return
			}
			ts.open, ts.team, ts.invite = t.ID, api.Team{}, api.Invite{}
			id := t.ID
			a.cloudDo("", func(cl *client.Client) error {
				var full api.Team
				err := cl.CloudCall("GET", teamPath(api.PathTeam, id), nil, &full)
				a.onMain(func() {
					if ts.open == id {
						ts.team = full
					}
				})
				return err
			}, nil)
		}
		if !open || ts.team.ID != t.ID {
			return
		}
		ui.Column(c).FillWidth().Gap(4).Padding(4, 12, 12, 12).Children(func() {
			for _, m := range ts.team.Members {
				a.cloudMemberRow(c, cs, m)
			}
			if t.Role == api.RoleOwner || t.Role == api.RoleAdmin {
				a.cloudInvite(c, cs)
			}
		})
	})
}

func (a *App) cloudMemberRow(c *ui.Context, cs *cloudSettings, m api.Member) {
	ts := &cs.teams
	ui.Row(c.Key("member:"+m.Account.ID)).FillWidth().Gap(10).AlignItems(ui.Center).Padding(6, 0).Children(func() {
		who := m.Account.Name
		if m.Account.GitHub != "" {
			who += " · @" + m.Account.GitHub
		}
		ui.Text(c, who).FontSize(12.5).Grow(1).Shrink(1).MinWidth(0).SingleLine()
		ui.Text(c, m.Role).FontSize(11.5).TextColor(theme.Muted)
		me := m.Account.ID == cs.st.AccountID
		canRemove := m.Role != api.RoleOwner && (me || ts.team.Role == api.RoleOwner || ts.team.Role == api.RoleAdmin)
		if !canRemove {
			return
		}
		label := "Remove " + m.Account.Name
		if me {
			label = "Leave the team"
		}
		if widget.IconButton(c, icon.Close, label, false).Clicked() {
			team, acc := ts.team.ID, m.Account.ID
			a.cloudDo("", func(cl *client.Client) error {
				return cl.CloudCall("DELETE", strings.Replace(teamPath(api.PathMember, team), "{account}", acc, 1), nil, nil)
			}, func() { ts.open = ""; a.loadCloud() })
		}
	})
}

// cloudInvite makes a one-time invite link to the open team, as a role.
func (a *App) cloudInvite(c *ui.Context, cs *cloudSettings) {
	ts := &cs.teams
	ui.Column(c).FillWidth().Gap(8).Padding(10, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(theme.Border).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Invite as").FontSize(12).TextColor(theme.Muted)
			if picked := widget.Segmented(c, []string{"Member", "Viewer", "Admin"}, ts.role, nil); picked >= 0 {
				ts.role = picked
			}
			ui.Spacer(c)
			if widget.Button(c, widget.Secondary, icon.Link, "Make a link", "").Clicked() {
				team, role := ts.team.ID, inviteRoles[ts.role]
				a.cloudDo("", func(cl *client.Client) error {
					var in api.Invite
					err := cl.CloudCall("POST", teamPath(api.PathInvites, team), api.InviteParams{Role: role}, &in)
					a.onMain(func() { ts.invite = in })
					return err
				}, nil)
			}
		})
		if in := ts.invite; in.Token != "" {
			link := in.URL
			if link == "" {
				link = in.Token
			}
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Padding(8, 10).Radius(theme.RadiusControl).Background(theme.Panel).Children(func() {
				ui.Text(c, link).Font(theme.MonoFont).FontSize(11.5).Grow(1).Shrink(1).MinWidth(0).SingleLine()
				widget.CopyIcon(c.Key("invite-copy"), &ts.copied, icon.Copy, "Copy the link", "Copy the invite link", link)
			})
			ui.Text(c, "For one person, as "+in.Role+", until "+in.Expires.Local().Format("2 Jan 15:04")+". It shows once: copy it now.").FontSize(11.5).TextColor(theme.Muted)
		}
	})
}
