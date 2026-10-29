package ui

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// A team's kit (docs/cloud.md): the skills, commands and subagents it
// shares, taken from the connected host's agents, and the features its
// projects' sessions get, in the team's dialog.

// hostTeamKit is the connected host's teams' kits, as last listed, and
// its agents' items to add from. It belongs to the main thread.
type hostTeamKit struct {
	host     string
	list     []proto.TeamKitItem
	items    []proto.EnvItem // the host's agents' skills, commands, subagents
	adding   bool            // the host's items show, to add
	filter   string          // narrows the host's items
	featured string          // narrows the features
	features map[string]map[string]bool
	err      string
}

func (a *App) teamKitOf() *hostTeamKit {
	if a.teamKitList == nil || a.teamKitList.host != a.host.ID {
		a.teamKitList = &hostTeamKit{host: a.host.ID, features: map[string]map[string]bool{}}
		a.loadTeamKit()
	}
	return a.teamKitList
}

func (a *App) loadTeamKit() {
	tk, cl := a.teamKitList, a.client
	if tk == nil || cl == nil {
		return
	}
	go func() {
		list, err := cl.TeamKit()
		if err != nil {
			return
		}
		a.onMain(func() { tk.list = list })
	}()
}

// teamKitEvent takes the kits as they changed.
func (a *App) teamKitEvent(data json.RawMessage) func() {
	var list []proto.TeamKitItem
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		if tk := a.teamKitList; tk != nil && tk.host == a.host.ID {
			tk.list = list
			tk.features = map[string]map[string]bool{} // read again
		}
	}
}

// cloudTeamKit is the team dialog's Kit tab.
func (a *App) cloudTeamKit(c *ui.Context, cs *cloudSettings) {
	tk := a.teamKitOf()
	team := cs.teams.team
	can := team.Role != "viewer"
	if tk.err != "" {
		widget.Notice(c, icon.Alert, theme.Error, tk.err, "", nil)
	}
	var mine []proto.TeamKitItem
	for _, it := range tk.list {
		if it.Team == team.ID {
			mine = append(mine, it)
		}
	}
	ui.Text(c, "Skills, commands and subagents the team shares: every session in its projects is told of them, whatever its agent.").FontSize(12).TextColor(theme.TextSoft)
	if len(mine) == 0 {
		ui.Text(c, "Nothing shared yet.").FontSize(12).TextColor(theme.Muted)
	}
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		for _, it := range mine {
			ui.Row(c.Key("kit:"+it.Kind+":"+it.Name)).FillWidth().Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
				widget.Tag(c, it.Kind, theme.TextSoft)
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
					ui.Text(c, it.Name).FontSize(12.5).SingleLine()
					if it.About != "" {
						ui.Text(c, it.About).FontSize(11).TextColor(theme.Muted).SingleLine()
					}
				})
				if can && widget.IconButton(c, icon.Close, "Take "+it.Name+" out of the kit", false).Clicked() {
					item := it
					a.confirm("Take "+item.Name+" out of the team's kit?", "Sessions in the team's projects stop being told of it. Your agents' own copy stays.", "Take it out", func() {
						cl := a.client
						go func() {
							if err := cl.RemoveFromTeamKit(item.Team, item.Kind, item.Name); err != nil {
								a.onMain(func() { tk.err = client.Explain(err) })
							}
						}()
					})
				}
			})
		}
	})
	if !can {
		return
	}
	if !tk.adding {
		if widget.Button(c, widget.Secondary, icon.Plus, "Share one of "+a.host.Name+"'s agents' skills…", "").Clicked() {
			tk.adding = true
			cl := a.client
			go func() {
				scan, err := cl.EnvScan()
				a.onMain(func() {
					if err != nil {
						tk.err = client.Explain(err)
						return
					}
					tk.items = nil
					for _, it := range scan.Items {
						if it.Kind == proto.EnvSkill || it.Kind == proto.EnvCommand || it.Kind == proto.EnvSubagent {
							tk.items = append(tk.items, it)
						}
					}
				})
			}()
		}
		return
	}
	widget.Field(c, a.host.Name+"'s agents", func() {
		if len(tk.items) == 0 {
			ui.Text(c, "No skills, commands or subagents here.").FontSize(12).TextColor(theme.Muted)
			return
		}
		widget.SearchField(c, &tk.filter, "Filter skills, commands, subagents", icon.Search)
		q := strings.ToLower(strings.TrimSpace(tk.filter))
		shown := 0
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, it := range tk.items {
				if q != "" && !strings.Contains(strings.ToLower(it.Name+" "+it.Kind+" "+a.agentName(it.Agent)), q) {
					continue
				}
				shown++
				shared := slices.ContainsFunc(mine, func(k proto.TeamKitItem) bool { return k.Kind == it.Kind && k.Name == it.Name })
				ui.Row(c.Key("host:"+it.ID)).FillWidth().Gap(8).AlignItems(ui.Center).Padding(4, 6).Children(func() {
					widget.Tag(c, it.Kind, theme.TextSoft)
					ui.Text(c, it.Name).FontSize(12.5).Grow(1).Shrink(1).MinWidth(0).SingleLine()
					ui.Text(c, a.agentName(it.Agent)).FontSize(11).TextColor(theme.Muted)
					switch {
					case shared:
						widget.Tag(c, "shared", theme.Done)
					case len(it.Secrets) > 0:
						widget.Tag(c, "holds secrets", theme.Warning)
					default:
						if widget.Button(c, widget.Secondary, nil, "Share", "").Clicked() {
							cl, team, id := a.client, team.ID, it.ID
							go func() {
								_, err := cl.AddToTeamKit(team, id)
								a.onMain(func() {
									tk.err = ""
									if err != nil {
										tk.err = client.Explain(err)
									}
								})
							}()
						}
					}
				})
			}
		})
		if shown == 0 {
			ui.Text(c, "Nothing matches “"+tk.filter+"”.").FontSize(12).TextColor(theme.Muted)
		}
	})
}

// cloudTeamFeatures are the team's switches for its projects' sessions,
// in its Options tab: as the host has it, on, or off.
func (a *App) cloudTeamFeatures(c *ui.Context, cs *cloudSettings) {
	tk := a.teamKitOf()
	team := cs.teams.team.ID
	cur, read := tk.features[team]
	if !read {
		tk.features[team] = nil // asked
		cl := a.client
		go func() {
			m, err := cl.TeamFeatures(team, nil, false)
			if err == nil {
				a.onMain(func() { tk.features[team] = m })
			}
		}()
	}
	widget.Field(c, "Features in the team's projects", func() {
		ui.Text(c, "Over each host's own switches; a project's own still win.").FontSize(11.5).TextColor(theme.Muted)
		widget.SearchField(c, &tk.featured, "Filter features", icon.Search)
		q := strings.ToLower(strings.TrimSpace(tk.featured))
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, f := range agent.Features {
				if f.Core || q != "" && !strings.Contains(strings.ToLower(f.Name+" "+f.About), q) {
					continue
				}
				mode := 0
				if on, set := cur[f.ID]; set {
					mode = 2
					if on {
						mode = 1
					}
				}
				ui.Row(c.Key("feature:"+f.ID)).FillWidth().Gap(8).AlignItems(ui.Center).Padding(3, 0).Children(func() {
					ui.Text(c, f.Name).FontSize(12.5).Grow(1).Shrink(1).MinWidth(0).SingleLine()
					if i := widget.Segmented(c, []string{"Host's", "On", "Off"}, mode, nil); i >= 0 {
						next := map[string]bool{}
						for k, v := range cur {
							next[k] = v
						}
						delete(next, f.ID)
						if i > 0 {
							next[f.ID] = i == 1
						}
						tk.features[team] = next
						cl := a.client
						go func() {
							if _, err := cl.TeamFeatures(team, next, true); err != nil {
								a.onMain(func() { tk.err = client.Explain(err) })
							}
						}()
					}
				})
			}
		})
	})
}
