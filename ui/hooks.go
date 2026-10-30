package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Settings → Hooks: what runs when something happens in a session, on
// every agent (proto.Hook), kept by the connected host's daemon; their
// log; projects' own hook files, run once the user allows them.

// hooksState is the section's. It belongs to the main thread.
type hooksState struct {
	host   string
	asked  bool
	loaded bool
	info   proto.HooksInfo
	log    []proto.HookRun
	err    string
	tab    int // 0 the hooks, 1 the log
	busy   bool

	form   hookForm
	review proto.ProjectHooks // the project file being reviewed
	seeing bool
}

// hookEventNames name the events as the section shows them.
var hookEventNames = map[string]string{
	proto.HookSessionStart: "Session starts",
	proto.HookPromptStart:  "Before a prompt",
	proto.HookToolBefore:   "Before a tool runs",
	proto.HookToolAfter:    "After a tool runs",
	proto.HookPermission:   "Agent asks permission",
	proto.HookWaiting:      "Agent waits for you",
	proto.HookTurnDone:     "Turn done",
	proto.HookFileChanged:  "Files changed",
	proto.HookCommit:       "Commit made",
	proto.HookSessionEnd:   "Session ends",
}

func (a *App) hooksOf() *hooksState {
	if a.hooks.host != a.host.ID || !a.hooks.asked {
		a.hooks = hooksState{host: a.host.ID, asked: true}
		a.loadHooks()
	}
	return &a.hooks
}

func (a *App) loadHooks() {
	cl, host := a.client, a.host.ID
	if cl == nil {
		return
	}
	go func() {
		info, err := cl.Hooks("")
		log, _ := cl.HookLog()
		a.onMain(func() {
			if a.hooks.host != host {
				return
			}
			a.hooks.loaded, a.hooks.info, a.hooks.log, a.hooks.err = true, info, log, ""
			if err != nil {
				a.hooks.err = client.Explain(err)
			}
		})
	}()
}

func (a *App) hooksSection(c *ui.Context) {
	st := a.hooksOf()
	sectionIntro(c, "Hooks", "Run a command, give the agent context or refuse what it is about to do when something happens in a session, on every agent: in the agent where it has hooks of its own (Claude Code, Codex), else as "+a.host.Name+" sees it happen.")
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		if i := widget.Segmented(c, []string{"Hooks", "Log"}, st.tab, nil); i >= 0 && i != st.tab {
			st.tab = i
			if i == 1 {
				a.loadHooks()
			}
		}
		ui.Spacer(c)
		if st.tab == 1 {
			if widget.GhostButton(c, "Refresh").Clicked() {
				a.loadHooks()
			}
			return
		}
		if widget.PrimaryButton(c, icon.Plus, "Add a hook", "").Disabled(!st.loaded).Clicked() {
			st.form.start(proto.Hook{On: true, Event: proto.HookToolBefore, Action: proto.HookActionRun})
		}
	})
	switch {
	case st.err != "":
		widget.Notice(c, icon.Alert, theme.Error, st.err, "", nil)
		return
	case !st.loaded:
		widget.Spinner(c, 14)
		return
	}
	if st.tab == 1 {
		a.hookLogList(c, st)
		return
	}
	for _, ph := range st.info.Pending {
		a.pendingHooks(c.Key("pending:"+ph.Dir), st, ph)
	}
	own := slices.DeleteFunc(slices.Clone(st.info.Hooks), func(h proto.Hook) bool { return h.Project != "" })
	if len(own) == 0 {
		ui.Text(c, "None yet. Refuse rm -rf and pushes to main, format the files an agent edits, add your project's rules to every prompt, or ping a channel when a turn is done.").
			FontSize(12).TextColor(theme.Muted)
	}
	for i, h := range own {
		a.hookRow(c.Key(h.ID), st, i, h)
	}
	a.hookFormSheet(c, st)
	a.hookReviewSheet(c, st)
}

// hookSummary is what hook h does, when, in a line.
func hookSummary(h proto.Hook) string {
	what := ""
	switch h.Action {
	case proto.HookActionRun:
		what = "runs " + firstLine(strings.TrimSpace(h.Command))
		if h.Background {
			what += " in the background"
		}
	case proto.HookActionContext:
		what = "adds context"
	case proto.HookActionBlock:
		what = "refuses"
	}
	var when []string
	if len(h.Agents) > 0 {
		when = append(when, strings.Join(h.Agents, ", "))
	}
	if len(h.Projects) > 0 {
		when = append(when, strings.Join(h.Projects, ", "))
	}
	if h.Tool != "" {
		when = append(when, "tool /"+h.Tool+"/")
	}
	if len(h.Paths) > 0 {
		when = append(when, strings.Join(h.Paths, ", "))
	}
	if h.Text != "" {
		when = append(when, "text /"+h.Text+"/")
	}
	out := hookEventNames[h.Event] + " → " + what
	if len(when) > 0 {
		out += " · only " + strings.Join(when, " · ")
	}
	return out
}

func (a *App) hookRow(c *ui.Context, st *hooksState, i int, h proto.Hook) {
	widget.Card(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, hookActionIcon(h.Action)).FontSize(15).TextColor(theme.TextSoft)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				name := h.Name
				if name == "" {
					name = hookEventNames[h.Event]
				}
				ui.Text(c, name).FontSize(12.5).SingleLine()
				ui.Text(c, hookSummary(h)).FontSize(11).TextColor(theme.Muted).SingleLine()
			})
			on := h.On
			if widget.Switch(c, &on, "On") {
				h.On = on
				a.saveHook(st, i, h)
			}
			widget.MoreMenu(c, &a.menu, "hook:"+h.ID, func(done func()) {
				if widget.MenuItem(c, icon.Pencil, "Edit…", false) {
					st.form.start(h)
					st.form.index = i
					done()
				}
				if widget.MenuItem(c, icon.Trash, "Remove", true) {
					a.confirm("Remove the hook “"+hookSummary(h)+"”?", "It stops running on every agent of "+a.host.Name+".", "Remove", func() { a.saveHook(st, i, proto.Hook{}) })
					done()
				}
			})
		})
	})
}

func hookActionIcon(action string) *ui.SVG {
	switch action {
	case proto.HookActionBlock:
		return icon.Lock
	case proto.HookActionContext:
		return icon.Chat
	}
	return icon.Terminal
}

// saveHook puts h at index i of the host's hooks (appended at -1, removed
// when h has no event) and saves them.
func (a *App) saveHook(st *hooksState, i int, h proto.Hook) {
	cl := a.client
	if cl == nil {
		return
	}
	list := slices.DeleteFunc(slices.Clone(st.info.Hooks), func(h proto.Hook) bool { return h.Project != "" })
	switch {
	case h.Event == "" && i >= 0 && i < len(list):
		list = slices.Delete(list, i, i+1)
	case i >= 0 && i < len(list):
		list[i] = h
	default:
		list = append(list, h)
	}
	st.busy = true
	go func() {
		_, err := cl.SaveHooks(list)
		a.onMain(func() {
			st.busy = false
			if err != nil {
				st.form.err = client.Explain(err)
				return
			}
			st.form.open = false
			a.loadHooks()
		})
	}()
}

// pendingHooks is a project's hook file waiting for the user.
func (a *App) pendingHooks(c *ui.Context, st *hooksState, ph proto.ProjectHooks) {
	n := len(ph.Hooks)
	what := fmt.Sprintf("%s has %d hook%s of its own", filepath.Base(ph.Dir), n, map[bool]string{true: "", false: "s"}[n == 1])
	widget.Notice(c, icon.Alert, theme.Warning, what, "They don't run until you allow them, and ask again whenever the project's "+".enowx/hooks.json changes.", func() {
		if widget.Button(c, widget.Secondary, nil, "Review…", "").Clicked() {
			st.review, st.seeing = ph, true
		}
	})
}

// hookReviewSheet shows a project's hooks before they may run.
func (a *App) hookReviewSheet(c *ui.Context, st *hooksState) {
	if !st.seeing {
		return
	}
	ph := st.review
	widget.FormSheet(c, &st.seeing, 560, "Hooks of "+filepath.Base(ph.Dir),
		"From "+filepath.Join(ph.Dir, ".enowx", "hooks.json")+". Allowed, they run like yours on "+a.host.Name+", commands included.", func() {
			for _, h := range ph.Hooks {
				ui.Column(c.Key(h.ID)).FillWidth().Gap(3).Padding(8, 10).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
					ui.Text(c, hookSummary(h)).FontSize(12)
					switch h.Action {
					case proto.HookActionRun:
						ui.Text(c, h.Command).Font(theme.MonoFont).FontSize(11.5).TextColor(theme.TextSoft)
					case proto.HookActionContext:
						ui.Text(c, h.Context).FontSize(11.5).TextColor(theme.TextSoft).MaxLines(6)
					case proto.HookActionBlock:
						ui.Text(c, h.Reason).FontSize(11.5).TextColor(theme.TextSoft)
					}
				})
			}
		}, func() {
			if widget.PrimaryButton(c, icon.Check, "Allow them", "").Disabled(st.busy).Clicked() {
				a.approveHooks(st, ph)
			}
		})
}

func (a *App) approveHooks(st *hooksState, ph proto.ProjectHooks) {
	cl := a.client
	if cl == nil {
		return
	}
	st.busy = true
	go func() {
		err := cl.ApproveHooks(proto.HooksApproval{Dir: ph.Dir, Hash: ph.Hash, Allow: true})
		a.onMain(func() {
			st.busy = false
			if err != nil {
				a.status = client.Explain(err)
				return
			}
			st.seeing = false
			a.loadHooks()
		})
	}()
}

// hookLogList is the latest runs of the host's hooks.
func (a *App) hookLogList(c *ui.Context, st *hooksState) {
	if len(st.log) == 0 {
		ui.Text(c, "No hook has run since "+a.host.Name+"'s daemon started.").FontSize(12).TextColor(theme.Muted)
		return
	}
	for i, r := range st.log {
		ui.Column(c.Key(i)).FillWidth().Gap(3).Padding(8, 10).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				col, label := theme.Done, "ok"
				switch {
				case r.Error != "":
					col, label = theme.Error, r.Error
				case r.Blocked:
					col, label = theme.Warning, "refused"
				case r.Exit != 0:
					col, label = theme.Error, fmt.Sprintf("exit %d", r.Exit)
				}
				ui.Box(c).Size(7, 7).Radius(4).Background(col)
				name := r.Name
				if name == "" {
					name = hookEventNames[r.Event]
				}
				ui.Text(c, name).FontSize(12).SingleLine().Shrink(1)
				ui.Text(c, hookEventNames[r.Event]).FontSize(11).TextColor(theme.Muted).SingleLine()
				ui.Spacer(c)
				meta := label + " · " + fmt.Sprintf("%d ms", r.Ms)
				if r.Agent != "" {
					meta = a.agentName(r.Agent) + " · " + meta
				}
				ui.Text(c, meta+" · "+r.Start.Local().Format("15:04:05")).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Muted).SingleLine()
			})
			if out := strings.TrimSpace(r.Output); out != "" {
				ui.Text(c, out).Font(theme.MonoFont).FontSize(11).TextColor(theme.TextSoft).MaxLines(4)
			}
		})
	}
}
