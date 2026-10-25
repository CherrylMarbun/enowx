package ui

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/chatview"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/notesview"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The Notes page's agent: a session of the agent the user picks for a
// note, on notes only (SessionCreateParams.NotesFor), drawn beside the
// page as a chat, or a terminal for agents without one. What it writes
// comes back as proposals the page shows.

// notesAgents are the agents the page offers: those installed.
func (a *App) notesAgents() []proto.AgentInfo {
	var out []proto.AgentInfo
	for _, ag := range a.agents {
		if ag.Command != "" && ag.Installed {
			out = append(out, ag)
		}
	}
	return out
}

// notesAgentOf is the agent note id talks to: the one picked for it last,
// else Claude Code when installed, else the first.
func (a *App) notesAgentOf(id string) string {
	agents := a.notesAgents()
	if pick := a.state.NotesAgents[id]; slices.ContainsFunc(agents, func(ag proto.AgentInfo) bool { return ag.ID == pick }) {
		return pick
	}
	if slices.ContainsFunc(agents, func(ag proto.AgentInfo) bool { return ag.ID == "claude" }) {
		return "claude"
	}
	if len(agents) > 0 {
		return agents[0].ID
	}
	return ""
}

// notesSession is note id's session of agent, the newest.
func (a *App) notesSession(id, agentID string) (proto.SessionInfo, bool) {
	for i := len(a.sessions) - 1; i >= 0; i-- {
		if s := a.sessions[i]; s.NotesFor == id && s.Agent == agentID && !a.removed[s.ID] {
			return s, true
		}
	}
	return proto.SessionInfo{}, false
}

// notesProposals is the proposals waiting, with their agents' names and
// the notes whose agents made them.
func (a *App) notesProposals() []notesview.Proposal {
	var out []notesview.Proposal
	for _, p := range a.noteList.proposals {
		pr := notesview.Proposal{NoteProposal: p, Agent: "An agent"}
		if s, ok := a.sessionByID(p.Session); ok {
			pr.Agent, pr.AgentID, pr.For = a.agentName(s.Agent), s.Agent, s.NotesFor
		}
		out = append(out, pr)
	}
	return out
}

// proposalsEvent takes the proposals as they changed.
func (a *App) proposalsEvent(data json.RawMessage) func() {
	var list []proto.NoteProposal
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		if hn := a.noteList; hn != nil && hn.host == a.host.ID {
			hn.proposals = list
		}
	}
}

// notesAssistant is the agent's panel for note n.
func (a *App) notesAssistant(c *ui.Context, n proto.NoteInfo) {
	agentID := a.notesAgentOf(n.ID)
	s, ok := a.notesSession(n.ID, agentID)
	ui.Row(c).FillWidth().Height(44).Shrink(0).PaddingX(12).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		pick := ui.ButtonBase(c).Height(30).PaddingX(8).Gap(6).Radius(theme.RadiusControl).Cursor(ui.CursorPointer).Label("Agent")
		if pick.Hovered() {
			pick.Background(theme.Hover)
		}
		pick.Children(func() {
			ui.Icon(c, icon.Agent(agentID)).FontSize(14)
			ui.Text(c, a.agentName(agentID)).FontSize(12.5).TextColor(theme.Text)
			ui.Icon(c, icon.ChevronDown).FontSize(10).TextColor(theme.Muted)
		})
		a.menu.OpenBelow(pick, "notes-agent", 30)
		a.menu.Show(c, pick, "notes-agent", func(done func()) {
			for _, ag := range a.notesAgents() {
				if widget.MenuItem(c.Key(ag.ID), icon.Agent(ag.ID), ag.Name, false) {
					if a.state.NotesAgents == nil {
						a.state.NotesAgents = map[string]string{}
					}
					a.state.NotesAgents[n.ID] = ag.ID
					done()
				}
			}
		})
		ui.Spacer(c)
		if ok {
			st := statusOf(s)
			widget.Dot(c, st.color, st.hollow, 7)
			ui.Text(c, st.label).FontSize(11.5).TextColor(theme.Muted).SingleLine()
			if widget.IconButton(c, icon.Plus, "New conversation", false).Clicked() {
				a.endNotesSession(s)
			}
		}
	})
	if agentID == "" {
		ui.Text(c, "No agent is installed on "+a.host.Name+".").FontSize(12).TextColor(theme.Muted).Padding(16)
		return
	}
	if !ok {
		a.notesStart(c, n, agentID)
		return
	}
	shown := func() bool { return a.page == pageNotes && a.notes.AssistantOpen() && a.notes.SelectedNote() == n.ID }
	switch p := a.panes[s.ID]; {
	case chatview.Supported(s):
		ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() { a.chatViewWhile(c.Key("chat:"+s.ID), s, shown) })
	case p != nil:
		terminal.View(c.Key("term:"+s.ID), p.term).Grow(1).FillWidth()
	default:
		ui.Text(c, "Connecting…").FontSize(12).TextColor(theme.Muted).Padding(16)
	}
}

// notesSuggestions are what the panel offers to ask before a conversation.
var notesSuggestions = []string{
	"Tidy this page: group it, make tasks to-dos",
	"Turn this into a plan: goal, steps, risks, next step",
	"Brainstorm this as a new project: pitch, MVP, open questions",
	"Summarize this page in a few lines at the top",
}

// notesStart asks the first question, which starts the agent.
func (a *App) notesStart(c *ui.Context, n proto.NoteInfo, agentID string) {
	ui.Column(c).Grow(1).FillWidth().Padding(16).Gap(10).Children(func() {
		ui.Text(c, "Ask "+a.agentName(agentID)+" about this page").FontSize(14).TextColor(theme.TextStrong)
		about := "It works on your notes only: no project, no files, no commands. What it writes shows above the page for you to accept or reject."
		if agentID != "claude" {
			about = "It runs in an empty folder and is asked to keep to your notes; Claude Code is held to them. What it writes shows above the page for you to accept or reject."
		}
		ui.Text(c, about).FontSize(11.5).TextColor(theme.Muted)
		for _, sug := range notesSuggestions {
			b := ui.ButtonBase(c.Key(sug)).FillWidth().Padding(8, 10).Radius(8).Border(1, theme.Border).Cursor(ui.CursorPointer).Label(sug)
			if b.Hovered() {
				b.Background(theme.Hover)
			}
			b.Children(func() { ui.Text(c, sug).FontSize(12).TextColor(theme.TextSoft) })
			if b.Clicked() {
				a.notesAsk(n, agentID, sug)
			}
		}
		ui.Spacer(c)
		send := false
		in := ui.TextAreaBase(c, &a.notesDraft).FillWidth().Lines(2, 8).Padding(10).Radius(8).Background(theme.Sunken).
			Border(1, theme.BorderStrong).FontSize(12.5).Placeholder("Ask anything about this page…").Label("Ask the agent")
		in.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEnter && k.Mods == 0 {
				send = true
				return true
			}
			return false
		})
		b := widget.PrimaryButton(c, icon.Send, "Send", "").AlignSelf(ui.End)
		if strings.TrimSpace(a.notesDraft) == "" {
			b.Disabled(true).Opacity(0.45)
		} else if b.Clicked() {
			send = true
		}
		if send && strings.TrimSpace(a.notesDraft) != "" {
			a.notesAsk(n, agentID, strings.TrimSpace(a.notesDraft))
			a.notesDraft = ""
		}
	})
}

// notesAsk starts note n's agent on a question about it.
func (a *App) notesAsk(n proto.NoteInfo, agentID, question string) {
	cl := a.client
	if cl == nil {
		return
	}
	title := n.Title
	if strings.TrimSpace(title) == "" {
		title = "Untitled"
	}
	prompt := "The page open beside you is “" + title + "” (note " + n.ID + "). " + question
	params := proto.SessionCreateParams{Agent: agentID, NotesFor: n.ID, Prompt: prompt, Cols: 100, Rows: 40}
	go func() {
		info, err := cl.CreateSession(params)
		a.onMain(func() {
			if err != nil {
				a.status = strings.TrimPrefix(err.Error(), proto.MethodSessionCreate+": ")
				return
			}
			a.upsert(info)
			if ch := a.chatOf(info.ID); ch != nil {
				ch.ScrollToEnd()
			}
		})
	}()
}

// endNotesSession ends a note's conversation, so the next question starts
// another.
func (a *App) endNotesSession(s proto.SessionInfo) {
	cl := a.client
	if cl == nil {
		return
	}
	a.remove(s.ID)
	go func() {
		if err := cl.KillSession(s.ID); err != nil {
			a.onMain(func() { a.status = err.Error() })
		}
	}()
}
