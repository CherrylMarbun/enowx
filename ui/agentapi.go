package ui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Settings' Model APIs: each agent of the host pointed at a model API of
// the user's (core/agentapi), its own settings backed up first and put
// back on asking.

type agentAPIState struct {
	host    string
	list    []proto.AgentAPIInfo
	loaded  bool
	busy    bool
	err     string
	edit    *agentAPIForm
	menus   map[string]*widget.ContextMenu
	showAll bool // the agents not installed too
}

// agentAPIForm is an agent's API being set.
type agentAPIForm struct {
	open                   bool
	agent                  proto.AgentAPIInfo
	url, key, model, small string
	protocol               int
	err                    string
}

func (a *App) agentAPIsOf() *agentAPIState {
	if a.agentAPIs == nil || a.agentAPIs.host != a.host.ID {
		a.agentAPIs = &agentAPIState{host: a.host.ID, menus: map[string]*widget.ContextMenu{}}
		a.loadAgentAPIs()
	}
	return a.agentAPIs
}

func (a *App) loadAgentAPIs() {
	st, cl := a.agentAPIs, a.client
	if cl == nil {
		return
	}
	go func() {
		list, err := cl.AgentAPIs()
		a.onMain(func() {
			st.loaded = true
			if err != nil {
				st.err = client.Explain(err)
				return
			}
			st.err, st.list = "", list
		})
	}()
}

// agentAPIDo runs a change off the main thread and shows the agent as it
// is after.
func (a *App) agentAPIDo(st *agentAPIState, do func(*client.Client) (proto.AgentAPIInfo, error), done func()) {
	cl := a.client
	if cl == nil || st.busy {
		return
	}
	st.busy, st.err = true, ""
	go func() {
		info, err := do(cl)
		a.onMain(func() {
			st.busy = false
			if err != nil {
				st.err = client.Explain(err)
				if st.edit != nil && st.edit.open {
					st.edit.err = st.err
				}
				return
			}
			for i := range st.list {
				if st.list[i].Agent == info.Agent {
					st.list[i] = info
				}
			}
			if done != nil {
				done()
			}
		})
	}()
}

func (a *App) agentAPISection(c *ui.Context) {
	st := a.agentAPIsOf()
	sectionIntro(c, "Model APIs", "Point an agent on "+a.host.Name+" at a model API of yours: a company gateway, OpenRouter, a local server. enowX writes it into the agent's own settings, the way that agent takes it, after keeping them as they were: Restore puts them back. Agents can do this too (enowx-app api).")
	switch {
	case st.err != "":
		widget.Notice(c, icon.Alert, theme.Error, st.err, "", nil)
	case !st.loaded:
		widget.Spinner(c, 14)
	}
	var shown, hidden []proto.AgentAPIInfo
	for _, ag := range st.list {
		if ag.Installed || ag.Current != nil || len(ag.Backups) > 0 || st.showAll {
			shown = append(shown, ag)
		} else {
			hidden = append(hidden, ag)
		}
	}
	for _, ag := range shown {
		a.agentAPIRow(c.Key(ag.Agent), st, ag)
	}
	if len(hidden) > 0 {
		ui.Row(c).FillWidth().Children(func() {
			if widget.GhostButton(c, "Show the "+strconv.Itoa(len(hidden))+" agents not installed").Clicked() {
				st.showAll = true
			}
		})
	}
	a.agentAPISheet(c, st)
}

func (a *App) agentAPIRow(c *ui.Context, st *agentAPIState, ag proto.AgentAPIInfo) {
	widget.Card(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			agentTile(c, ag.Agent, 24)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, ag.Name).FontSize(13)
				detail := "Its own API"
				if cur := ag.Current; cur != nil {
					detail = cur.BaseURL
					if cur.Model != "" {
						detail += " · " + cur.Model
					}
					if cur.APIKey != "" {
						detail += " · key " + cur.APIKey
					}
				}
				col := theme.Muted
				if ag.Current != nil {
					col = theme.TextSoft
				}
				ui.Text(c, detail).FontSize(11.5).TextColor(col).SingleLine().Shrink(1)
				if ag.Error != "" {
					ui.Text(c, ag.Error).FontSize(11).TextColor(theme.Error)
				}
			})
			label := "Use an API…"
			if ag.Current != nil {
				label = "Change…"
			}
			if widget.Button(c, widget.Secondary, nil, label, "").Disabled(st.busy).Clicked() {
				f := &agentAPIForm{open: true, agent: ag}
				if cur := ag.Current; cur != nil {
					f.url, f.model, f.small = cur.BaseURL, cur.Model, cur.SmallModel
					f.protocol = max(0, slices.Index(ag.Protocols, cur.Protocol))
				}
				st.edit = f
			}
			if ag.Current != nil || len(ag.Backups) > 0 {
				m := st.menus[ag.Agent]
				if m == nil {
					m = &widget.ContextMenu{}
					st.menus[ag.Agent] = m
				}
				widget.MoreMenu(c, m, "api:"+ag.Agent, func(done func()) {
					if ag.Current != nil && widget.MenuItem(c, icon.Restore, "Back to its own API", false) {
						a.agentAPIDo(st, func(cl *client.Client) (proto.AgentAPIInfo, error) { return cl.ClearAgentAPI(ag.Agent) }, nil)
						done()
					}
					if len(ag.Backups) > 0 {
						widget.MenuSeparator(c)
						widget.PopupLabel(c, "Restore its settings as they were")
					}
					for _, b := range ag.Backups {
						label := b.Time.Local().Format("Jan 2 15:04") + " · before " + strings.TrimPrefix(b.Note, "restoring ")
						if b.Original {
							label = "As you had them (" + b.Time.Local().Format("Jan 2 15:04") + ")"
						}
						if widget.MenuItem(c, icon.Restore, label, false) {
							id := b.ID
							a.agentAPIDo(st, func(cl *client.Client) (proto.AgentAPIInfo, error) { return cl.RestoreAgentAPI(ag.Agent, id) }, nil)
							done()
						}
					}
				})
			}
		})
	})
}

func (a *App) agentAPISheet(c *ui.Context, st *agentAPIState) {
	f := st.edit
	if f == nil || !f.open {
		st.edit = nil
		return
	}
	ag := f.agent
	about := "Written into " + strings.Join(ag.Files, ", ") + " on " + a.host.Name + ", which are kept first. Sessions of " + ag.Name + " started afterwards use it."
	widget.FormSheet(c, &f.open, 560, ag.Name+": a model API", about, func() {
		widget.Field(c, "Base URL", func() {
			widget.Input(c, &f.url, "https://openrouter.ai/api/v1").FillWidth().Font(theme.MonoFont)
		})
		if len(ag.Protocols) > 1 {
			widget.Field(c, "It speaks", func() {
				labels := make([]string, len(ag.Protocols))
				for i, p := range ag.Protocols {
					labels[i] = map[string]string{"anthropic": "Anthropic", "openai": "OpenAI (chat)", "openai-responses": "OpenAI (responses)", "gemini": "Gemini"}[p]
				}
				ui.Row(c).Children(func() {
					if i := widget.Segmented(c, labels, f.protocol, nil); i >= 0 {
						f.protocol = i
					}
				})
			})
		}
		key := "API key"
		if ag.Current != nil && ag.Current.APIKey != "" {
			key = "API key (empty keeps " + ag.Current.APIKey + ")"
		}
		widget.Field(c, key, func() {
			widget.Input(c, &f.key, "sk-…").FillWidth().Password()
		})
		ui.Row(c).FillWidth().Gap(10).Children(func() {
			if ag.NoModel {
				return
			}
			ui.Column(c).Grow(1).Children(func() {
				widget.Field(c, "Model", func() { widget.Input(c, &f.model, "anthropic/claude-sonnet").FillWidth().Font(theme.MonoFont) })
			})
			if ag.Small {
				ui.Column(c).Grow(1).Children(func() {
					widget.Field(c, "Model for quick work (optional)", func() { widget.Input(c, &f.small, "").FillWidth().Font(theme.MonoFont) })
				})
			}
		})
		if ag.Note != "" {
			ui.Text(c, ag.Note).FontSize(11.5).TextColor(theme.Muted)
		}
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12).TextColor(theme.Error)
		}
	}, func() {
		label := "Use it"
		if st.busy {
			label = "Saving…"
		}
		if widget.PrimaryButton(c, icon.Check, label, "").Disabled(st.busy || strings.TrimSpace(f.url) == "").Clicked() {
			api := proto.AgentAPI{BaseURL: f.url, APIKey: f.key, Model: f.model, SmallModel: f.small}
			if f.protocol < len(ag.Protocols) {
				api.Protocol = ag.Protocols[f.protocol]
			}
			if api.APIKey == "" && ag.Current != nil {
				api.APIKey = proto.AgentAPIKeepKey // the daemon keeps the key it has
			}
			id := ag.Agent
			a.agentAPIDo(st, func(cl *client.Client) (proto.AgentAPIInfo, error) { return cl.SetAgentAPI(id, api) }, func() { f.open = false })
		}
	})
}
