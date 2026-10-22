package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// A session shows as its terminal or as a chat read from the transcript
// its agent records. The terminal keeps running behind the chat: typing in
// the chat types into it, and approvals or menus still show there.

// chatState is a session's chat. It belongs to the main thread; its
// poller reports through win.Update.
type chatState struct {
	entries   []proto.TranscriptEntry
	rev       int64
	supported bool
	reason    string
	loaded    bool
	polling   bool
	input     string
	open      map[int]bool // expanded tool calls and thoughts, by index
	scroll    bool         // scroll to the end on the next frame
}

// chatPoll is how often a shown chat asks for news.
const chatPoll = 700 * time.Millisecond

// hasChat reports whether the session's agent could show as a chat.
func hasChat(s proto.SessionInfo) bool {
	_, ok := agent.Find(s.Agent)
	return ok
}

func (a *App) chatOf(id string) *chatState {
	if a.chats == nil {
		a.chats = map[string]*chatState{}
	}
	ch := a.chats[id]
	if ch == nil {
		ch = &chatState{open: map[int]bool{}}
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
	a.chatOf(s.ID).scroll = true
}

func (a *App) showsChat(id string) bool { return a.state.Chats[id] }

// pollChat follows the transcript while the chat shows.
func (a *App) pollChat(id string, ch *chatState) {
	if ch.polling || a.client == nil {
		return
	}
	ch.polling = true
	cl, gen := a.client, a.gen
	go func() {
		since := ch.rev
		for {
			t, err := cl.Transcript(id, since)
			stop := make(chan bool, 1)
			a.win.Update(func() {
				if gen != a.gen || !a.showsChat(id) || a.chats[id] != ch {
					ch.polling = false
					stop <- true
					return
				}
				stop <- false
				if err != nil {
					ch.reason, ch.loaded = err.Error(), true
					return
				}
				ch.apply(t)
			})
			if <-stop {
				return
			}
			if err == nil {
				since = t.Rev
			}
			time.Sleep(chatPoll)
		}
	}()
}

func (ch *chatState) apply(t proto.Transcript) {
	ch.loaded, ch.supported, ch.reason = true, t.Supported, t.Reason
	if t.Reset {
		ch.entries, ch.open = nil, map[int]bool{}
	}
	grew := false
	for _, e := range t.Entries {
		switch {
		case e.Index < len(ch.entries):
			ch.entries[e.Index] = e
		case e.Index == len(ch.entries):
			ch.entries = append(ch.entries, e)
			grew = true
		}
	}
	ch.rev = t.Rev
	if grew {
		ch.scroll = true
	}
}

// chatView draws the chat in place of the terminal.
func (a *App) chatView(c *ui.Context, s proto.SessionInfo) {
	ch := a.chatOf(s.ID)
	a.pollChat(s.ID, ch)
	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		sc := ui.Scroll(c).Grow(1).FillWidth().MinHeight(0)
		sc.Children(func() {
			ui.Column(c).FillWidth().Padding(14, 16).Gap(12).Children(func() {
				switch {
				case !ch.loaded:
					ui.Text(c, "Reading the conversation…").TextColor(colMuted)
				case len(ch.entries) == 0:
					ui.Text(c, ch.reason).TextColor(colMuted)
				}
				for i := range ch.entries {
					a.chatEntry(c.Key(i), ch, ch.entries[i])
				}
				end := ui.Box(c).Height(1).FillWidth()
				if ch.scroll {
					end.ScrollIntoView()
					ch.scroll = false
				}
			})
		})
		if ch.supported || !ch.loaded {
			a.chatInput(c, s, ch)
		}
	})
}

func (a *App) chatEntry(c *ui.Context, ch *chatState, e proto.TranscriptEntry) {
	switch e.Kind {
	case proto.TranscriptUser:
		ui.Row(c).FillWidth().Children(func() {
			ui.Box(c).Grow(1).MinWidth(40)
			ui.Column(c).Shrink(1).MaxWidth(560).Padding(9, 12).Radius(12).Background(ui.Hex("#1F2740")).Children(func() {
				ui.Text(c, e.Text).TextColor(colTextStrong).Selectable()
			})
		})
	case proto.TranscriptAssistant:
		markdown(c, e.Text)
	case proto.TranscriptThinking:
		a.foldable(c, ch, e.Index, "Thinking", "", "", colMuted, func() {
			ui.Text(c, e.Text).FontSize(12).TextColor(colMuted).Selectable()
		})
	case proto.TranscriptTool:
		status, col := "…", colTextSoft
		switch {
		case e.Failed:
			status, col = "✕", colError
		case e.Done:
			status, col = "✓", colDone
		}
		a.foldable(c, ch, e.Index, e.Text, e.Detail, status, col, func() {
			if e.Result == "" {
				ui.Text(c, "No output yet.").FontSize(12).TextColor(colMuted)
				return
			}
			ui.Text(c, e.Result).Font(monoFont).FontSize(11).TextColor(colTextSoft).Selectable()
		})
	case proto.TranscriptNotice:
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Text(c, e.Text).FontSize(11).TextColor(colMuted)
		})
	}
}

// foldable is a one-line summary that opens to show more.
func (a *App) foldable(c *ui.Context, ch *chatState, index int, title, detail, status string, statusColor ui.Color, body func()) {
	open := ch.open[index]
	ui.Column(c).FillWidth().Radius(8).Border(1, colBorder).Background(ui.Hex("#111318")).Children(func() {
		head := ui.ButtonBase(c).FillWidth().Gap(8).Padding(6, 10).Cursor(ui.CursorPointer).Justify(ui.Start).Label(title)
		if head.Hovered() {
			head.Background(colHover)
		}
		head.Children(func() {
			chev := iconChevronRight
			if open {
				chev = iconChevronDown
			}
			ui.Icon(c, chev).FontSize(11).TextColor(colMuted)
			ui.Text(c, title).FontSize(12).Bold().TextColor(colTextSoft).SingleLine().Shrink(0)
			if detail != "" {
				ui.Text(c, detail).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine().Shrink(1).Grow(1)
			} else {
				ui.Box(c).Grow(1)
			}
			if status != "" {
				ui.Text(c, status).FontSize(12).TextColor(statusColor)
			}
		})
		if head.Clicked() {
			ch.open[index] = !open
		}
		if open {
			ui.Column(c).FillWidth().Padding(4, 12, 10, 12).Children(body)
		}
	})
}

// chatInput types a message into the session's terminal.
func (a *App) chatInput(c *ui.Context, s proto.SessionInfo, ch *chatState) {
	ui.Column(c).FillWidth().Gap(6).Padding(10, 12).BorderWidth(1, 0, 0, 0).BorderColor(colBorder).Background(colPanel).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			in := ui.TextInput(c, &ch.input).Placeholder("Message " + a.agentName(s.Agent) + "…").Label("Message").Grow(1)
			send := primaryButton(c, iconCheck, "Send", "")
			if (in.Submitted() || send.Clicked()) && strings.TrimSpace(ch.input) != "" {
				a.sendToSession(s.ID, ch.input)
				ch.input, ch.scroll = "", true
			}
		})
		ui.Text(c, "Approvals and menus show in the terminal.").FontSize(11).TextColor(colMuted)
	})
}

// sendToSession types a line into the session, then Enter. The input has
// one line, so no paste markers are needed, which programs that never
// turned paste mode on would show as text.
func (a *App) sendToSession(id, text string) {
	p := a.panes[id]
	if p == nil {
		a.status = "The session isn't attached yet; try again in a moment."
		return
	}
	go func() {
		_, _ = p.stream.Write([]byte(text))
		// Agents that read a burst of input as a paste would take an Enter
		// sent with it as a newline; a short gap keeps it a keypress.
		time.Sleep(50 * time.Millisecond)
		_, _ = p.stream.Write([]byte("\r"))
	}()
}
