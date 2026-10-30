package ui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// hookForm adds a hook or changes one, in a dialog. Its lists are typed
// as text, comma-separated.
type hookForm struct {
	open                    bool
	hook                    proto.Hook
	index                   int // in the host's hooks, -1 for a new one
	agents, projects, paths string
	timeout                 string
	err                     string
	testing                 bool
	tested                  *proto.HookRun
}

var hookActions = []string{proto.HookActionRun, proto.HookActionContext, proto.HookActionBlock}

func (f *hookForm) start(h proto.Hook) {
	*f = hookForm{open: true, hook: h, index: -1,
		agents: strings.Join(h.Agents, ", "), projects: strings.Join(h.Projects, ", "), paths: strings.Join(h.Paths, ", ")}
	if h.Timeout > 0 {
		f.timeout = strconv.Itoa(h.Timeout)
	}
}

// built is the form's hook, its text lists split.
func (f *hookForm) built() proto.Hook {
	h := f.hook
	h.Agents, h.Projects, h.Paths = splitList(f.agents), splitList(f.projects), splitList(f.paths)
	h.Timeout, _ = strconv.Atoi(strings.TrimSpace(f.timeout))
	h.Name = strings.TrimSpace(h.Name)
	return h
}

func (a *App) hookFormSheet(c *ui.Context, st *hooksState) {
	f := &st.form
	if !f.open {
		return
	}
	title := "New hook"
	if f.index >= 0 {
		title = "Edit hook"
	}
	widget.FormSheet(c, &f.open, 600, title, "Runs on "+a.host.Name+", for every agent that sees the event.", func() {
		h := &f.hook
		if f.err != "" {
			widget.Notice(c, icon.Alert, theme.Error, f.err, "", nil)
		}
		widget.Field(c, "Name", func() { widget.Input(c, &h.Name, "No rm -rf").FillWidth().AutoFocus() })
		widget.Field(c, "When", func() {
			sel := ui.SelectBase(c, &h.Event)
			widget.Trigger(sel.Trigger, sel.Open()).FillWidth().Label("When").Children(func() {
				ui.Text(c, hookEventNames[h.Event]).FontSize(12.5).SingleLine()
				ui.Spacer(c)
				ui.Icon(c, icon.ChevronDown).FontSize(12).TextColor(theme.Muted)
			})
			sel.Popup(func(panel ui.Element) {
				widget.Popup(panel.Margin(6, 0, 0, 0).Width(300))
				for _, e := range proto.HookEvents {
					item := sel.Item(e)
					widget.PopupItem(item, item.Highlighted()).Children(func() { ui.Text(c, hookEventNames[e]).FontSize(12.5) })
				}
			})
			ui.Text(c, a.hookReach(st, h.Event)).FontSize(11).TextColor(theme.Muted)
		})
		widget.Field(c, "Do", func() {
			cur := slices.Index(hookActions, h.Action)
			if i := widget.Segmented(c, []string{"Run a command", "Add context", "Refuse"}, cur, nil); i >= 0 {
				h.Action = hookActions[i]
			}
			switch h.Action {
			case proto.HookActionRun:
				ui.TextAreaBase(c, &h.Command).FillWidth().MinHeight(60).MaxHeight(160).Padding(9, 11).FontSize(12).Font(theme.MonoFont).
					Label("Command").Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border).
					Placeholder(`gofmt -w $(jq -r '.paths[]' )`)
				ui.Text(c, "Runs in the session's folder, the event as JSON on stdin. Exiting 2 refuses (stderr the reason); what it prints otherwise is given to the agent before a prompt or at a session's start.").
					FontSize(11).TextColor(theme.Muted)
				switchRow(c, &h.Background, "In the background", "The agent doesn't wait for it, so it can neither refuse nor add context.")
			case proto.HookActionContext:
				ui.TextAreaBase(c, &h.Context).FillWidth().MinHeight(70).MaxHeight(180).Padding(9, 11).FontSize(12.5).
					Label("Context").Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border).
					Placeholder("Answer in Indonesian. Run make lint before saying it's done.")
			case proto.HookActionBlock:
				widget.Input(c, &h.Reason, "Not on this project: ask me first").FillWidth()
				ui.Text(c, "Told to the agent as why.").FontSize(11).TextColor(theme.Muted)
			}
			if h.Action != proto.HookActionRun || !h.Background {
				if note := hookActionNote(h.Event, h.Action); note != "" {
					ui.Text(c, note).FontSize(11).TextColor(theme.Warning)
				}
			}
		})
		widget.Field(c, "Only when", func() {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				hookCondition(c, "Agents", &f.agents, "claude, codex")
				hookCondition(c, "Projects", &f.projects, "enowx, ~/code/site")
				hookCondition(c, "Tool or command", &h.Tool, `^Bash$|git push.*main`)
				hookCondition(c, "Files", &f.paths, "*.go, src/**")
				hookCondition(c, "Prompt or command text", &h.Text, `deploy|release`)
			})
			ui.Text(c, "Empty means any. Tool and text are regular expressions, files globs from the project's folder.").FontSize(11).TextColor(theme.Muted)
		})
		if h.Action == proto.HookActionRun {
			widget.Field(c, "Timeout (seconds)", func() { widget.Input(c, &f.timeout, "30").Width(120) })
		}
		if r := f.tested; r != nil {
			a.hookTestResult(c, r)
		}
	}, func() {
		if widget.Button(c, widget.Secondary, icon.Play, "Test", "").Disabled(f.testing).Clicked() {
			a.testHookForm(f)
		}
		if widget.PrimaryButton(c, icon.Check, "Save", "").Disabled(st.busy).Clicked() {
			f.err = ""
			a.saveHook(st, f.index, f.built())
		}
	})
}

func hookCondition(c *ui.Context, label string, value *string, placeholder string) {
	ui.Row(c.Key(label)).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(theme.TextSoft).Width(150).Shrink(0)
		widget.Input(c, value, placeholder).Grow(1).Font(theme.MonoFont)
	})
}

// hookActionNote warns of an action an event can't take.
func hookActionNote(event, action string) string {
	answers := event == proto.HookToolBefore || event == proto.HookPromptStart || event == proto.HookSessionStart
	switch {
	case action == proto.HookActionBlock && event != proto.HookToolBefore && event != proto.HookPromptStart:
		return "Only a tool about to run or a prompt can be refused: this one will just be logged."
	case action == proto.HookActionContext && !answers:
		return "Context reaches the agent only before a prompt or at a session's start."
	}
	return ""
}

// hookReach says which agents get event, and how.
func (a *App) hookReach(st *hooksState, event string) string {
	var in, out []string
	for id, events := range st.info.Support {
		switch events[event] {
		case proto.HookInAgent:
			in = append(in, a.agentName(id))
		case proto.HookOutside:
			out = append(out, id)
		}
	}
	slices.Sort(in)
	var parts []string
	if len(in) > 0 {
		parts = append(parts, "In the agent: "+strings.Join(in, ", "))
	}
	switch {
	case len(out) > 0 && len(in) > 0:
		parts = append(parts, "the others seen from outside, after the fact")
	case len(out) > 0:
		parts = append(parts, "Every agent, seen from outside, after the fact")
	}
	if len(parts) == 0 {
		return "Only agents with hooks of their own report this, and none do here."
	}
	return strings.Join(parts, "; ") + "."
}

// testHookForm runs the form's hook on a sample event.
func (a *App) testHookForm(f *hookForm) {
	cl := a.client
	if cl == nil {
		return
	}
	h := f.built()
	h.On = true
	ev := proto.HookEvent{Event: h.Event, Session: "test", Agent: "claude", Prompt: "Fix the login redirect", Tool: "Bash", Command: "go test ./...", Paths: []string{"main.go"}, Time: time.Now()}
	if p, ok := a.currentProject(); ok {
		ev.Dir, ev.Project = p.Path, p.Path
	}
	f.testing, f.tested = true, nil
	go func() {
		r, err := cl.TestHook(proto.HookTest{Hook: h, Event: ev})
		a.onMain(func() {
			f.testing = false
			if err != nil {
				f.err = client.Explain(err)
				return
			}
			f.err, f.tested = "", &r
		})
	}()
}

func (a *App) hookTestResult(c *ui.Context, r *proto.HookRun) {
	col, what := theme.Done, "Ran in "+strconv.FormatInt(r.Ms, 10)+" ms"
	switch {
	case r.Error != "":
		col, what = theme.Error, r.Error
	case r.Blocked:
		col, what = theme.Warning, "Refused"
	case r.Exit != 0:
		col, what = theme.Error, "Exited with "+strconv.Itoa(r.Exit)
	}
	ui.Column(c).FillWidth().Gap(4).Padding(8, 10).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
		ui.Text(c, "Test: "+what).FontSize(12).TextColor(col)
		if out := strings.TrimSpace(r.Output); out != "" {
			ui.Text(c, out).Font(theme.MonoFont).FontSize(11).TextColor(theme.TextSoft).MaxLines(8)
		}
	})
}
