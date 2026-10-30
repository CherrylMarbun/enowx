package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/accounts"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// An agent's card in Settings → Agent accounts: its accounts in the order
// they're used, each its state and what it has left, moved up or down,
// made the default, kept to some projects; what a limit does; adding a
// sign-in or an API key.

// accountStatus is account name of agentID's state, as the host told.
func (st *accountsState) accountStatus(agentID, name string) (proto.AccountStatus, bool) {
	i := slices.IndexFunc(st.info.Accounts, func(s proto.AccountStatus) bool { return s.Agent == agentID && s.Name == name })
	if i < 0 {
		return proto.AccountStatus{}, false
	}
	return st.info.Accounts[i], true
}

func (a *App) agentAccounts(c *ui.Context, st *accountsState, ag proto.AgentInfo) {
	pr := st.info.Prefs[ag.ID]
	var names []string
	for _, acc := range st.accountsOfAgent(ag.ID) {
		names = append(names, acc.Name)
	}
	order := accounts.Ordered(pr, names)
	widget.Card(c).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				agentTile(c, ag.ID, 24)
				ui.Text(c, ag.Name).FontSize(13).Grow(1)
				if st.naming[ag.ID] == nil && widget.GhostButton(c, "Add account").Clicked() {
					name := ""
					st.naming[ag.ID] = &name
				}
				if st.keying[ag.ID] == nil && widget.GhostButton(c, "Add API key").Tooltip("An account that is a key kept in the vault, lent to its sessions").Clicked() {
					st.keying[ag.ID] = &keyForm{}
				}
			})
			for i, name := range order {
				a.accountLine(c.Key("acc:"+name), st, ag, pr, order, i, name)
			}
			a.addAccountForms(c, st, ag)
			a.fallbackControls(c, st, ag, pr)
		})
	})
}

// accountLine is one account in the order: its place, name, state and
// what it has left, and what to do with it.
func (a *App) accountLine(c *ui.Context, st *accountsState, ag proto.AgentInfo, pr proto.AccountPrefs, order []string, i int, name string) {
	acc, isOther := proto.Account{}, name != ""
	if isOther {
		for _, x := range st.accountsOfAgent(ag.ID) {
			if x.Name == name {
				acc = x
			}
		}
	}
	status, _ := st.accountStatus(ag.ID, name)
	ui.Column(c).FillWidth().Gap(4).Padding(6, 8).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprint(i+1)).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).Width(14)
			label := name
			detail := "Its own sign-in"
			switch {
			case !isOther:
				label, detail = "Default", "The account "+ag.Name+" is signed in with on its own"
			case acc.Cred != "":
				detail = "API key " + acc.Cred + " from the vault"
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).FontSize(12.5)
					if pr.Default == name && (name != "" || pr.Default == "") && len(order) > 1 {
						widget.Tag(c, "new sessions", theme.Accent)
					}
					accountStateTag(c, status)
				})
				ui.Text(c, detail).FontSize(11).TextColor(theme.Muted).SingleLine()
				if cr := status.Credits; cr != nil {
					ui.Text(c, creditLine(*cr)).FontSize(11).TextColor(theme.TextSoft).SingleLine()
				}
				if only := pr.Only[name]; len(only) > 0 {
					ui.Text(c, "Only for "+strings.Join(only, ", ")).FontSize(11).TextColor(theme.TextSoft).SingleLine()
				}
			})
			if widget.IconButton(c.Key("up"), icon.ChevronUp, "Use it sooner", false).Disabled(i == 0).Clicked() && i > 0 {
				next := slices.Clone(order)
				next[i-1], next[i] = next[i], next[i-1]
				pr.Order = next
				a.saveAccountPrefs(st, ag.ID, pr)
			}
			if widget.IconButton(c.Key("down"), icon.ChevronDown, "Use it later", false).Disabled(i == len(order)-1).Clicked() && i < len(order)-1 {
				next := slices.Clone(order)
				next[i+1], next[i] = next[i], next[i+1]
				pr.Order = next
				a.saveAccountPrefs(st, ag.ID, pr)
			}
			widget.MoreMenu(c, &a.menu, "account:"+ag.ID+":"+name, func(done func()) {
				if widget.MenuItem(c, icon.Check, "Use for new sessions", false) {
					pr.Default = name
					a.saveAccountPrefs(st, ag.ID, pr)
					done()
				}
				if isOther && widget.MenuItem(c, icon.Folder, "Only for some projects…", false) {
					v := strings.Join(pr.Only[name], ", ")
					st.only[ag.ID+"\x00"+name] = &v
					done()
				}
				if isOther && acc.Cred == "" && widget.MenuItem(c, icon.Account, "Sign in", false) {
					a.signInAccount(ag.ID, acc)
					done()
				}
				if isOther && widget.MenuItem(c, icon.Trash, "Remove", true) {
					a.confirm("Remove the account "+name+"?", "Its sign-in on "+a.host.Name+" goes with it; sessions using it keep running until they end.", "Remove", func() { a.removeAccount(st, acc) })
					done()
				}
			})
		})
		if v := st.only[ag.ID+"\x00"+name]; v != nil {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				in := widget.Input(c, v, "acme, ~/code/client-site").Grow(1).Font(theme.MonoFont).AutoFocus()
				if widget.GhostButton(c, "Cancel").Clicked() {
					delete(st.only, ag.ID+"\x00"+name)
				}
				if widget.Button(c, widget.Secondary, nil, "Keep", "").Clicked() || in.Submitted() {
					if pr.Only == nil {
						pr.Only = map[string][]string{}
					}
					pr.Only[name] = splitList(*v)
					if len(pr.Only[name]) == 0 {
						delete(pr.Only, name)
					}
					delete(st.only, ag.ID+"\x00"+name)
					a.saveAccountPrefs(st, ag.ID, pr)
				}
			})
		}
	})
}

// accountStateTag says an account's limit, if it hit one.
func accountStateTag(c *ui.Context, s proto.AccountStatus) {
	switch s.State {
	case proto.AccountLimit:
		label := "at its limit"
		if !s.Until.IsZero() {
			label += " until " + s.Until.Local().Format("15:04")
			if time.Until(s.Until) > 20*time.Hour {
				label = "at its limit until " + s.Until.Local().Format("Jan 2 15:04")
			}
		}
		widget.Tag(c, label, theme.Warning)
	case proto.AccountCredits:
		widget.Tag(c, "out of credits", theme.Error)
	case proto.AccountAuth:
		widget.Tag(c, "signed out", theme.Error)
	}
}

// creditLine is what an account has left, in a line.
func creditLine(cr proto.CreditAccount) string {
	if cr.Error != "" {
		return cr.Error
	}
	var parts []string
	if cr.Plan != "" {
		parts = append(parts, cr.Plan)
	}
	for _, w := range cr.Windows {
		p := fmt.Sprintf("%s %.0f%% used", w.Label, w.Used)
		if !w.ResetsAt.IsZero() {
			p += ", resets " + w.ResetsAt.Local().Format("Mon 15:04")
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " · ")
}

// addAccountForms add a sign-in, or an API key of the vault's.
func (a *App) addAccountForms(c *ui.Context, st *accountsState, ag proto.AgentInfo) {
	if name := st.naming[ag.ID]; name != nil {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			in := widget.Input(c, name, "Work").Grow(1).AutoFocus()
			if widget.GhostButton(c, "Cancel").Clicked() {
				delete(st.naming, ag.ID)
			}
			ok := strings.TrimSpace(*name) != "" && !st.busy
			if (widget.PrimaryButton(c, nil, "Add and sign in", "").Disabled(!ok).Clicked() || in.Submitted()) && ok {
				a.addAccount(st, ag.ID, *name)
			}
		})
	}
	if k := st.keying[ag.ID]; k != nil {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			widget.Input(c, &k.name, "Team key").Width(160)
			widget.Input(c, &k.cred, "the vault's credential, e.g. anthropic-work").Grow(1).Font(theme.MonoFont)
			if widget.GhostButton(c, "Cancel").Clicked() {
				delete(st.keying, ag.ID)
			}
			ok := strings.TrimSpace(k.name) != "" && strings.TrimSpace(k.cred) != "" && !st.busy
			if widget.PrimaryButton(c, nil, "Add", "").Disabled(!ok).Clicked() && ok {
				a.addKeyAccount(st, ag.ID, *k)
			}
		})
		ui.Text(c, "Its value stays in the vault: sessions on this account get it as the vault lends credentials, never shown.").FontSize(11).TextColor(theme.Muted)
	}
}

// fallbackControls set what an account at its limit does.
func (a *App) fallbackControls(c *ui.Context, st *accountsState, ag proto.AgentInfo, pr proto.AccountPrefs) {
	ui.Column(c).FillWidth().Gap(6).Padding(4, 0, 0, 0).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "At a limit").FontSize(12).TextColor(theme.TextSoft).Width(110)
			modes := []string{proto.FallbackOff, proto.FallbackAsk, proto.FallbackAuto}
			cur := max(0, slices.Index(modes, pr.Fallback))
			if i := widget.Segmented(c, []string{"Stop and tell me", "Ask me first", "Go on as the next"}, cur, nil); i >= 0 && i != cur {
				pr.Fallback = modes[i]
				a.saveAccountPrefs(st, ag.ID, pr)
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Then").FontSize(12).TextColor(theme.TextSoft).Width(110)
			options, names := []string{""}, map[string]string{"": "nothing more"}
			for _, other := range a.agents {
				if other.Installed && other.ID != ag.ID && other.Command != "" && other.ID != "shell" {
					options = append(options, other.ID)
					names[other.ID] = "go on with " + other.Name
				}
			}
			to := pr.ToAgent
			hookSelect(c.Key("then:"+ag.ID), "Then", &to, options, names)
			if to != pr.ToAgent {
				pr.ToAgent = to
				a.saveAccountPrefs(st, ag.ID, pr)
			}
		})
		back := pr.Return
		if switchRowKey(c, "return:"+ag.ID, &back, "Back to the first once its limit resets", "Between turns, the session goes back to the account first in the order.") {
			pr.Return = back
			a.saveAccountPrefs(st, ag.ID, pr)
		}
		ui.Text(c, ag.Name+"'s terms are for your own accounts: don't share one, or use several to get around limits its plan doesn't allow. enowX only switches between the accounts you add here.").
			FontSize(11).TextColor(theme.Warning)
	})
}

func (a *App) saveAccountPrefs(st *accountsState, agentID string, pr proto.AccountPrefs) {
	cl := a.client
	if cl == nil {
		return
	}
	if st.info.Prefs == nil {
		st.info.Prefs = map[string]proto.AccountPrefs{}
	}
	st.info.Prefs[agentID] = pr // shown at once
	go func() {
		info, err := cl.SetAccountPrefs(agentID, pr)
		a.onMain(func() {
			if err != nil {
				st.err = client.Explain(err)
				return
			}
			st.info = info
		})
	}()
}

func (a *App) addKeyAccount(st *accountsState, agentID string, k keyForm) {
	cl := a.client
	if cl == nil {
		return
	}
	st.busy = true
	go func() {
		_, err := cl.AddKeyAccount(agentID, strings.TrimSpace(k.name), strings.TrimSpace(k.cred))
		a.onMain(func() {
			st.busy = false
			if err != nil {
				st.err = client.Explain(err)
				return
			}
			delete(st.keying, agentID)
			a.loadAccounts()
		})
	}()
}

// accountHistory is which account each session used, when, and why.
func (a *App) accountHistory(c *ui.Context, st *accountsState) {
	if len(st.log) == 0 {
		ui.Text(c, "Nothing yet: sessions started, switched and fallen back show here.").FontSize(12).TextColor(theme.Muted)
		return
	}
	for i, e := range st.log[:min(len(st.log), 200)] {
		ui.Row(c.Key(i)).FillWidth().Gap(10).Padding(6, 8).Radius(theme.RadiusControl).Background(theme.Sunken).AlignItems(ui.Center).Children(func() {
			ui.Text(c, e.Time.Local().Format("Jan 2 15:04")).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).Width(96)
			col := theme.TextSoft
			switch e.Event {
			case "fallback", "limit":
				col = theme.Warning
			case "back":
				col = theme.Done
			}
			ui.Text(c, e.Event).FontSize(11.5).TextColor(col).Width(64)
			name := e.Account
			if name == "" {
				name = "Default"
			}
			ui.Text(c, a.agentName(e.Agent)+" · "+name).FontSize(12).SingleLine().Width(200)
			ui.Text(c, e.Reason).FontSize(11).TextColor(theme.Muted).SingleLine().Grow(1).Shrink(1)
			if e.Session != "" {
				ui.Text(c, a.hookSessionName(e.Session)).FontSize(11).TextColor(theme.Muted).SingleLine().MaxWidth(160)
			}
		})
	}
}
