package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/chatview"
)

// The chats of sessions shown as a chat (see package chatview), and which
// sessions show as one, remembered between launches.

func (a *App) chatOf(id string) *chatview.Chat {
	if a.chats == nil {
		a.chats = map[string]*chatview.Chat{}
	}
	ch := a.chats[id]
	if ch == nil {
		ch = chatview.New()
		a.chats[id] = ch
	}
	return ch
}

func (a *App) toggleChat(s proto.SessionInfo) {
	if a.state.Chats == nil {
		a.state.Chats = map[string]bool{}
	}
	if a.state.Chats[s.ID] {
		delete(a.state.Chats, s.ID)
		return
	}
	a.state.Chats[s.ID] = true
	ch := a.chatOf(s.ID)
	ch.ScrollToEnd()
	ch.FocusComposer()
}

func (a *App) showsChat(id string) bool { return a.state.Chats[id] }

// chatView draws the chat in place of the terminal.
func (a *App) chatView(c *ui.Context, s proto.SessionInfo) {
	ch := a.chatOf(s.ID)
	if cl := a.client; cl != nil {
		gen := a.gen
		ch.Follow(cl, s.ID, a.win.Update, func() bool {
			return gen == a.gen && a.showsChat(s.ID) && a.chats[s.ID] == ch
		})
	}
	ch.View(c, chatview.Options{
		Agent:     s.Agent,
		AgentName: a.agentName(s.Agent),
		Working:   isWorking(s),
		Send:      func(text string) { a.sendToSession(s.ID, text) },
		Interrupt: func() { a.typeIntoSession(s.ID, "\x1b") },
		Set:       func(setting, id string) { a.setAgentSetting(s, setting, id) },
	})
}

// setAgentSetting asks session s's agent for a setting, as its preset
// says: a command typed in for the model and effort, the daemon cycling
// the agent's mode key for the mode.
func (a *App) setAgentSetting(s proto.SessionInfo, setting, id string) {
	set := agent.SettingsOf(s.Agent)
	switch setting {
	case "model":
		a.sendToSession(s.ID, fmt.Sprintf(set.ModelCommand, id))
	case "effort":
		a.sendToSession(s.ID, fmt.Sprintf(set.EffortCommand, id))
	case "mode":
		cl := a.client
		if cl == nil {
			return
		}
		go func() {
			if err := cl.SwitchMode(s.ID, id); err != nil {
				a.win.Update(func() { a.status = err.Error() })
			}
		}()
	}
}

// typeIntoSession types raw keys into the session.
func (a *App) typeIntoSession(id, keys string) {
	if p := a.panes[id]; p != nil {
		go func() { _, _ = p.stream.Write([]byte(keys)) }()
	}
}

// sendToSession types text into the session, then Enter. Text of several
// lines goes as a paste, so its line breaks don't send it early.
func (a *App) sendToSession(id, text string) {
	p := a.panes[id]
	if p == nil {
		a.status = "The session isn't attached yet; try again in a moment."
		return
	}
	go func() {
		if strings.Contains(text, "\n") {
			text = "\x1b[200~" + text + "\x1b[201~"
		}
		_, _ = p.stream.Write([]byte(text))
		// Agents that read a burst of input as a paste would take an Enter
		// sent with it as a newline; a short gap keeps it a keypress.
		time.Sleep(50 * time.Millisecond)
		_, _ = p.stream.Write([]byte("\r"))
	}()
}
