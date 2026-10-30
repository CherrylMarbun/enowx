package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/accounts"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Settings → Agent accounts: an agent's other accounts on the connected
// host (proto.Account), each signed in once and picked per session.

// accountsState is the section's. It belongs to the main thread.
type accountsState struct {
	host   string
	asked  bool
	loaded bool
	list   []proto.Account
	info   proto.AccountsInfo // states, what they have left, preferences
	log    []proto.AccountLogEntry
	tab    int // 0 the accounts, 1 the history
	err    string
	naming map[string]*string // agent → the name of the account being added
	keying map[string]*keyForm
	only   map[string]*string // account ID → its projects, being set
	busy   bool
}

// keyForm adds an account that is an API key: its name and the vault's
// credential.
type keyForm struct{ name, cred string }

func (a *App) accountsOf() *accountsState {
	if a.accts.host != a.host.ID || !a.accts.asked {
		a.accts = accountsState{host: a.host.ID, asked: true, naming: map[string]*string{}, keying: map[string]*keyForm{}, only: map[string]*string{}}
		a.loadAccounts()
	}
	return &a.accts
}

func (a *App) loadAccounts() {
	cl, host := a.client, a.host.ID
	if cl == nil {
		return
	}
	go func() {
		list, err := cl.Accounts()
		info, ierr := cl.AccountsInfo()
		log, _ := cl.AccountLog()
		a.onMain(func() {
			if a.accts.host != host {
				return
			}
			a.accts.loaded, a.accts.list, a.accts.err = true, list, ""
			if ierr == nil {
				a.accts.info, a.accts.log = info, log
			}
			if err != nil {
				a.accts.err = client.Explain(err)
			}
		})
	}()
}

// accountAgents are the installed agents that can have accounts.
func (a *App) accountAgents() []proto.AgentInfo {
	var out []proto.AgentInfo
	for _, ag := range a.agents {
		if ag.Installed && agent.Base(ag.ID) == ag.ID && accounts.Supported(ag.ID) {
			out = append(out, ag)
		}
	}
	return out
}

// accountsOfAgent are agent's other accounts (agent may be a variant, as
// Claude Agent Teams, of the agent they sign in).
func (st *accountsState) accountsOfAgent(id string) []proto.Account {
	var out []proto.Account
	for _, acc := range st.list {
		if acc.Agent == agent.Base(id) {
			out = append(out, acc)
		}
	}
	return out
}

func (a *App) accountsSection(c *ui.Context) {
	st := a.accountsOf()
	sectionIntro(c, "Agent accounts", "Other accounts of an agent on "+a.host.Name+", each signed in once (or an API key from the vault), used in the order you set: a session at one's usage limit can go on as the next. They share the agent's settings, skills and conversations; only the sign-in differs.")
	ui.Row(c).FillWidth().Children(func() {
		if i := widget.Segmented(c, []string{"Accounts", "History"}, st.tab, nil); i >= 0 && i != st.tab {
			st.tab = i
			a.loadAccounts()
		}
	})
	switch {
	case st.err != "":
		widget.Notice(c, icon.Alert, theme.Error, st.err, "", nil)
	case !st.loaded:
		widget.Spinner(c, 14)
	}
	if st.tab == 1 {
		a.accountHistory(c, st)
		return
	}
	agents := a.accountAgents()
	if st.loaded && len(agents) == 0 {
		ui.Text(c, "Claude Code and Codex can have other accounts; neither is installed on "+a.host.Name+".").FontSize(12).TextColor(theme.Muted)
	}
	for _, ag := range agents {
		a.agentAccounts(c.Key(ag.ID), st, ag)
	}
}

// addAccount keeps a new account of agent and starts signing it in.
func (a *App) addAccount(st *accountsState, agentID, name string) {
	cl := a.client
	if cl == nil {
		return
	}
	st.busy = true
	go func() {
		acc, err := cl.AddAccount(agentID, name)
		a.onMain(func() {
			st.busy = false
			if err != nil {
				st.err = client.Explain(err)
				return
			}
			delete(st.naming, agentID)
			a.loadAccounts()
			a.signInAccount(agentID, acc)
		})
	}()
}

// signInAccount starts a session signing agent in as acc.
func (a *App) signInAccount(agentID string, acc proto.Account) {
	cl := a.client
	if cl == nil {
		return
	}
	p, _ := a.currentProject()
	go func() {
		info, err := cl.CreateSession(proto.SessionCreateParams{Project: p.Path, Agent: agentID, SignIn: true, Account: acc.ID,
			Name: "Sign in " + acc.Name, BranchMode: proto.BranchNone, Cols: 120, Rows: 36})
		a.onMain(func() {
			if err != nil {
				a.status = "Couldn't start signing in: " + client.Explain(err)
				return
			}
			a.upsert(info)
			a.selectSession(info)
		})
	}()
}

func (a *App) removeAccount(st *accountsState, acc proto.Account) {
	cl := a.client
	if cl == nil {
		return
	}
	st.busy = true
	go func() {
		err := cl.RemoveAccount(acc.ID)
		a.onMain(func() {
			st.busy = false
			if err != nil {
				st.err = client.Explain(err)
			}
			a.loadAccounts()
		})
	}()
}

// accountField picks the account the new session's agent signs in as,
// when it has others.
func (a *App) accountField(c *ui.Context) {
	f := &a.newSession
	list := a.accountsOf().accountsOfAgent(f.agent)
	if len(list) == 0 {
		f.account = ""
		return
	}
	if !f.accountPicked {
		f.account = a.defaultAccountFor(f.agent, f.project.Path) // by the preferences, until picked
	}
	widget.Field(c, "Account", func() {
		labels := []string{"Default"}
		cur := 0
		for i, acc := range list {
			labels = append(labels, acc.Name)
			if acc.ID == f.account {
				cur = i + 1
			}
		}
		if i := widget.Segmented(c, labels, cur, nil); i >= 0 {
			f.account = ""
			if i > 0 {
				f.account = list[i-1].ID
			}
		}
	})
}
