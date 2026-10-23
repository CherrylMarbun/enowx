package ui

import (
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/cli"
	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
	"github.com/enowx/enowx/version"
)

// Settings' sections beside the agents' library: what new sessions start
// with, the variables every session on a host gets, the app's own
// preferences, and what a host's daemon says about itself.

const (
	sectionLibrary     = "Agents' library"
	sectionSessions    = "New sessions"
	sectionEnvironment = "Environment"
	sectionApp         = "App"
	sectionDiagnostics = "Diagnostics"
)

var settingsSections = []string{sectionLibrary, sectionSessions, sectionEnvironment, sectionApp, sectionDiagnostics}

// hostSettings is what the sections ask the connected host, read when
// they show.
type hostSettings struct {
	host     string // the host ID it is of
	vars     []proto.SessionEnvVar
	loaded   bool
	maxAgent int
	sleep    int // HibernateAfter, in minutes
	log      proto.DaemonLog
	err      string
	newName  string
	newValue string
}

// hostSettingsOf is the sections' state for the connected host, read anew
// when the host changed.
func (a *App) hostSettingsOf() *hostSettings {
	if a.hostSet == nil || a.hostSet.host != a.host.ID {
		a.hostSet = &hostSettings{host: a.host.ID}
		a.loadHostSettings()
	}
	return a.hostSet
}

func (a *App) loadHostSettings() {
	hs, cl := a.hostSet, a.client
	if cl == nil {
		return
	}
	go func() {
		vars, err := cl.SessionEnv()
		st, _ := cl.Settings()
		log, _ := cl.DaemonLog(80)
		a.onMain(func() {
			hs.loaded = true
			if err != nil {
				hs.err = client.Explain(err)
			}
			hs.vars, hs.maxAgent, hs.sleep, hs.log = vars, st.MaxAgents, st.HibernateAfter, log
		})
	}()
}

// settingsBody is the section shown.
func (a *App) settingsBody(c *ui.Context) {
	if a.settingsSection == "" || a.settingsSection == sectionLibrary {
		a.library.Draw(c, a.libraryOptions())
		return
	}
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(720).AlignSelf(ui.Center).Padding(20).Gap(16).Children(func() {
			switch a.settingsSection {
			case sectionSessions:
				a.sessionDefaults(c)
			case sectionEnvironment:
				a.environmentSection(c)
			case sectionApp:
				a.appSection(c)
			case sectionDiagnostics:
				a.diagnosticsSection(c)
			}
		})
	})
}

// sessionDefaults are what New session starts with.
func (a *App) sessionDefaults(c *ui.Context) {
	sectionIntro(c, "New sessions", "What New session starts with; each session can still differ.")
	widget.Field(c, "Agent", func() {
		ui.Row(c).FillWidth().Gap(6).Wrap().Children(func() {
			for _, ag := range a.agents {
				if ag.ID == "custom" || ag.Command != "" && !ag.Installed {
					continue
				}
				on := a.defaultAgent() == ag.ID
				b := ui.ButtonBase(c.Key(ag.ID)).Height(30).PaddingX(10).Gap(7).Radius(theme.RadiusControl).Border(1, theme.Border).
					Cursor(ui.CursorPointer).Label(ag.Name)
				if on {
					b.Border(1, theme.AccentLine).Background(theme.Selected)
				}
				b.Children(func() {
					ui.Icon(c, icon.Agent(ag.ID)).FontSize(13)
					ui.Text(c, ag.Name).FontSize(12.5)
				})
				if b.Clicked() {
					a.state.DefaultAgent = ag.ID
				}
			}
		})
	})
	widget.Field(c, "In a git project", func() {
		cur := 0
		if a.state.ProjectFolder {
			cur = 1
		}
		if i := widget.Segmented(c, []string{"A new branch and worktree", "The project folder"}, cur, nil); i >= 0 {
			a.state.ProjectFolder = i == 1
		}
	})
	if switchRow(c, &a.skipPermissions, "Agents skip permission prompts", "They run tools without asking; the chat can still switch it per session.") {
		a.saveState()
	}
	widget.Field(c, "Time budget for agents", func() {
		labels := []string{"None"}
		for _, m := range budgetChoices[1:] {
			labels = append(labels, workTime(int64(m)*60))
		}
		cur := slices.Index(budgetChoices, a.state.DefaultBudget)
		if i := widget.Segmented(c, labels, max(cur, 0), nil); i >= 0 {
			a.state.DefaultBudget = budgetChoices[i]
		}
	})
	hs := a.hostSettingsOf()
	n := "No limit"
	if hs.maxAgent > 0 {
		n = strconv.Itoa(hs.maxAgent) + " at most; the others wait their turn"
	}
	widget.Row(c, "Agents at once on "+a.host.Name, "", n, func() {
		if widget.GhostButton(c, "Change…").Clicked() {
			a.askAgentLimit()
		}
	})
	widget.Field(c, "Hibernate idle agents on "+a.host.Name, func() {
		ui.Text(c, "An agent done with its turn and left idle this long is stopped to free its memory; its session stays, and it comes back on its conversation when shown. Claude Code, Codex and other agents enowX can continue.").
			FontSize(11.5).TextColor(theme.Muted)
		labels := make([]string, len(sleepChoices))
		for i, m := range sleepChoices {
			labels[i] = "Never"
			if m > 0 {
				labels[i] = workTime(int64(m) * 60)
			}
		}
		cur := max(slices.Index(sleepChoices, hs.sleep), 0)
		if i := widget.Segmented(c, labels, cur, nil); i >= 0 && sleepChoices[i] != hs.sleep {
			a.setHibernation(sleepChoices[i])
		}
	})
}

// sleepChoices are the idle minutes hibernation is offered at; 0 never.
var sleepChoices = []int{0, 15, 30, 60, 240}

// setHibernation sets the connected host's hibernation window, keeping its
// other settings.
func (a *App) setHibernation(minutes int) {
	hs, cl := a.hostSettingsOf(), a.client
	if cl == nil {
		return
	}
	hs.sleep = minutes
	go func() {
		st, err := cl.Settings()
		if err == nil {
			st.HibernateAfter = minutes
			_, err = cl.SetSettings(st)
		}
		if err != nil {
			a.onMain(func() { a.status = client.Explain(err) })
		}
	}()
}

// environmentSection keeps the variables every session on the host gets.
func (a *App) environmentSection(c *ui.Context) {
	hs := a.hostSettingsOf()
	sectionIntro(c, "Environment", "Variables every session on "+a.host.Name+" gets, agents included: API keys, tokens. Kept on "+a.host.Name+" in ~/.enowx/session-env.json, readable by its user alone; they apply to sessions started or continued after.")
	if hs.err != "" {
		ui.Text(c, hs.err).FontSize(12).TextColor(theme.Error)
	}
	for _, v := range hs.vars {
		ui.Row(c.Key(v.Name)).FillWidth().Gap(10).Padding(8, 10).Radius(6).Background(theme.Sunken).Children(func() {
			ui.Text(c, v.Name).Font(theme.MonoFont).FontSize(12.5).Grow(1)
			ui.Text(c, v.Value).Font(theme.MonoFont).FontSize(12).TextColor(theme.Muted)
			if widget.IconButton(c, icon.Close, "Remove it", false).Clicked() {
				a.setVar(v.Name, "")
			}
		})
	}
	ui.Row(c).FillWidth().Gap(6).Children(func() {
		ui.Column(c).Width(220).Children(func() { widget.Input(c, &hs.newName, "ANTHROPIC_API_KEY").FillWidth().Font(theme.MonoFont) })
		ui.Column(c).Grow(1).Children(func() {
			ui.TextInputBase(c, &hs.newValue).Password().FillWidth().Height(theme.ControlHeight).PaddingX(11).Radius(theme.RadiusControl).
				Background(theme.Sunken).Border(1, theme.BorderStrong).Placeholder("value").Label("Value")
		})
		if widget.Button(c, widget.Secondary, icon.Plus, "Set", "").Clicked() && strings.TrimSpace(hs.newName) != "" && hs.newValue != "" {
			a.setVar(strings.TrimSpace(hs.newName), hs.newValue)
			hs.newName, hs.newValue = "", ""
		}
	})
	if a.host.ID != hosts.LocalID {
		ui.Text(c, "Setting one sends its value to "+a.host.Name+".").FontSize(11.5).TextColor(theme.Warning)
	}
}

func (a *App) setVar(name, value string) {
	hs, cl := a.hostSet, a.client
	if cl == nil {
		return
	}
	go func() {
		vars, err := cl.SetSessionEnv(name, value)
		a.onMain(func() {
			if err != nil {
				hs.err = client.Explain(err)
				return
			}
			hs.vars, hs.err = vars, ""
		})
	}()
}

// appSection is the app's own preferences.
func (a *App) appSection(c *ui.Context) {
	sectionIntro(c, "App", "enowX on this device.")
	widget.Field(c, "Editor", func() {
		names := make([]string, len(editors))
		for i, e := range editors {
			names[i] = e.name
		}
		cur := slices.IndexFunc(editors, func(e editor) bool { return e.id == a.preferredEditor().id })
		if i := widget.Segmented(c, names, cur, nil); i >= 0 {
			a.state.Editor = editors[i].id
		}
	})
	notify := !a.state.NotifyOff
	if switchRow(c, &notify, "Notifications", "When an agent needs input or finishes in a session not shown.") {
		a.state.NotifyOff = !notify
	}
	auto := !a.state.Updates.Manual
	if switchRow(c, &auto, "Look for updates by itself", "At start and every few hours; Check for updates looks now.") {
		a.state.Updates.Manual = !auto
	}
	ui.Row(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "enowX "+version.String).FontSize(12.5).TextColor(theme.TextSoft)
		if widget.GhostButton(c, "Check for updates").Clicked() {
			a.checkForUpdates(true)
		}
	})
	a.commandLineRow(c)
	widget.Group(c, "Keyboard shortcuts", len(shortcutList()), &a.shortcutsOpen, func() {
		for _, s := range shortcutList() {
			ui.Row(c.Key(s[0])).FillWidth().Gap(10).Padding(4, 8).Children(func() {
				ui.Text(c, s[1]).FontSize(12.5).Grow(1)
				ui.Text(c, s[0]).Font(theme.MonoFont).FontSize(12).TextColor(theme.Muted)
			})
		}
	})
}

// commandLineRow offers to install the enowx command (docs/cli.md) as a
// link to this app, for shells and scripts.
func (a *App) commandLineRow(c *ui.Context) {
	exe, err := os.Executable()
	if err != nil || runtime.GOOS == "windows" {
		return
	}
	link, _ := cli.LinkPath()
	ui.Row(c).FillWidth().Gap(12).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, "Command line").FontSize(12.5)
			note := "enowx ls, spawn, send, wait… in any terminal: a link to this app at " + link + "."
			if cli.Linked(exe) {
				note = "Installed at " + link + "."
				if !cli.OnPath() {
					note += " Add ~/.local/bin to your PATH to run it by name."
				}
			}
			ui.Text(c, note).FontSize(11.5).TextColor(theme.Muted)
		})
		label := "Install"
		if cli.Linked(exe) {
			label = "Reinstall"
		}
		if widget.GhostButton(c, label).Clicked() {
			if _, err := cli.InstallLink(exe); err != nil {
				a.status = err.Error()
			}
		}
	})
}

// shortcutList is the app's keyboard shortcuts, as the keys and what they
// do.
func shortcutList() [][2]string {
	return [][2]string{
		{modKey + "K", "Command palette"},
		{modKey + "P", "Go to a file of the session"},
		{modKey + "N", "New session"},
		{modKey + "O", "Open a folder"},
		{modKey + "B", "Expand or collapse the sidebar"},
		{modKey + "⇧D", "Show or hide the side panel"},
		{modKey + "⇧P", "Browser tool"},
		{modKey + "⇧L", "Sessions side by side"},
		{modKey + "⇧]  " + modKey + "⇧[", "Next / previous session"},
		{modKey + "⇧↩", "Maximize the session"},
		{modKey + "-click", "Open a path or a local page printed in a terminal"},
	}
}

// diagnosticsSection says what the connected host's daemon is, and its
// log; it restarts it.
func (a *App) diagnosticsSection(c *ui.Context) {
	hs := a.hostSettingsOf()
	info := a.daemonInfo
	sectionIntro(c, "Diagnostics", "What enowX's daemon on "+a.host.Name+" says about itself.")
	rows := [][2]string{
		{"Host", a.host.Name + " · " + info.Hostname},
		{"Daemon", "enowxd " + info.Version + " · pid " + strconv.Itoa(info.PID)},
		{"Since", info.Started.Format("2 Jan 15:04") + " · " + textfmt.Age(info.Started, time.Now())},
		{"Sessions", strconv.Itoa(len(a.sessions))},
		{"Home", info.Home},
	}
	if info.Hash != "" {
		rows = append(rows, [2]string{"Build", info.Hash[:min(len(info.Hash), 12)]})
	}
	if !info.Display && info.DisplayNote != "" {
		rows = append(rows, [2]string{"Displays", info.DisplayNote})
	}
	widget.Card(c).Children(func() {
		ui.Grid(c).FillWidth().Columns(2).Gap(10).Children(func() {
			for _, r := range rows {
				ui.Column(c.Key(r[0])).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, r[0]).FontSize(11).TextColor(theme.Muted)
					ui.Text(c, r[1]).FontSize(12.5).SingleLine().Tooltip(r[1])
				})
			}
		})
	})
	widget.Row(c, "Restart the daemon", "", "Its sessions continue, as after an update.", func() {
		if widget.Button(c, widget.Secondary, icon.Sync, "Restart", "").Clicked() {
			a.restartDaemon()
		}
	})
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "LOG").FontSize(10.5).TextColor(theme.Muted)
		ui.Text(c, hs.log.Path).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Idle).SingleLine().Shrink(1)
		ui.Spacer(c)
		if widget.IconButton(c, icon.Sync, "Read the log again", false).Clicked() {
			a.loadHostSettings()
		}
	})
	widget.LogView(c, hs.log.Text, 300)
}

// sectionIntro heads a Settings section: what it is about.
func sectionIntro(c *ui.Context, title, about string) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		ui.Text(c, title).FontSize(16)
		ui.Text(c, about).FontSize(12.5).TextColor(theme.Muted)
	})
}

// restartDaemon restarts the connected host's daemon, and connects again.
func (a *App) restartDaemon() {
	cl, h := a.client, a.host
	if cl == nil {
		return
	}
	go func() {
		err := cl.RestartDaemon()
		time.Sleep(time.Second) // it hangs up its sessions first
		a.onMain(func() {
			if err != nil {
				a.status = client.Explain(err)
				return
			}
			a.notify("Restarted enowxd on " + h.Name)
			a.connect(h)
		})
	}()
}

// switchRow is a setting turned on or off: its name and what it does, the
// switch at the right. It reports a change.
func switchRow(c *ui.Context, on *bool, label, about string) bool {
	changed := false
	ui.Row(c.Key(label)).FillWidth().Gap(12).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, label).FontSize(12.5)
			if about != "" {
				ui.Text(c, about).FontSize(11.5).TextColor(theme.Muted)
			}
		})
		changed = widget.Switch(c, on, label)
	})
	return changed
}
