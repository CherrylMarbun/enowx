package ui

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// A team's chat (docs/cloud.md), the side panel's Team group: the team of
// the selected session's project, else the space shown. Its messages come
// through the team's space, read again every few seconds while it shows;
// "@" in a message mentions the team's members, notes, canvases, projects
// and the files of the session's project (teamchatcompose.go).

// panelChat is the side panel's chat tool.
const panelChat = "chat"

// chatRefresh is how often an open chat takes what the team sent.
const chatRefresh = 5 * time.Second

// membersRefresh is how often members shown are read again: who joined,
// left or changed role shows without reconnecting.
const membersRefresh = 15 * time.Second

// hostChats is the connected host's teams' chats, as last read, and the
// composer's draft. It belongs to the main thread.
type hostChats struct {
	host     string
	byTeam   map[string][]proto.ChatMessage
	list     ui.ListState // the messages, following the latest
	draft    string
	mentions []proto.ChatMention // picked for the draft, kept while "@Title" is in it
	pulled   time.Time
	err      string

	members    map[string][]api.Member // by team, read from the cloud
	membersAt  map[string]time.Time    // when each team's were last asked for
	membersErr map[string]string

	// The "@" being typed: its menu's pick, the draft it was closed for,
	// and the caret to put back after a completion.
	editing *proto.ChatMessage // the user's message the draft changes, not a new one
	saved   string             // the draft before editing began, put back after

	pick      int
	dismissed string
	setCaret  int // -1: none
	focus     bool

	// Files of the session's project found for a query, as last asked.
	files      []string
	filesFor   string
	filesAsked string
}

func (a *App) chatsOf() *hostChats {
	if a.teamChats == nil || a.teamChats.host != a.host.ID {
		a.teamChats = &hostChats{host: a.host.ID, byTeam: map[string][]proto.ChatMessage{},
			members: map[string][]api.Member{}, membersAt: map[string]time.Time{}, membersErr: map[string]string{}, setCaret: -1}
	}
	return a.teamChats
}

// chatTeam is the team the panel's Team group shows.
func (a *App) chatTeam() string {
	if s, ok := a.selectedSession(); ok {
		if p, ok := a.teamOfProject(s.Project); ok {
			return p.Team
		}
	}
	if t := a.space(); t != "" {
		return t
	}
	if list := a.spacesOf().list; len(list) > 0 {
		return list[0].Team
	}
	return ""
}

// loadChat reads a team's chat from the host.
func (a *App) loadChat(team string) {
	hc, cl := a.chatsOf(), a.client
	if cl == nil {
		return
	}
	go func() {
		list, err := cl.TeamChat(team, 0)
		a.onMain(func() {
			if err != nil {
				hc.err = client.Explain(err)
				return
			}
			hc.byTeam[team] = list
		})
	}()
}

// teamMembers is team's members as the cloud last told, read again when
// asked for after membersRefresh.
func (a *App) teamMembers(team string) []api.Member {
	hc, cl := a.chatsOf(), a.client
	if time.Since(hc.membersAt[team]) > membersRefresh && cl != nil {
		hc.membersAt[team] = time.Now()
		go func() {
			var full api.Team
			err := cl.CloudCall("GET", teamPath(api.PathTeam, team), nil, &full)
			a.onMain(func() {
				if err != nil {
					hc.membersErr[team] = client.Explain(err)
					return
				}
				hc.members[team], hc.membersErr[team] = full.Members, ""
			})
		}()
	}
	return hc.members[team]
}

// teamChatEvent takes messages that came or were sent.
func (a *App) teamChatEvent(data json.RawMessage) func() {
	var list []proto.ChatMessage
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		hc, me := a.chatsOf(), a.cloudOf().st.AccountID
		for _, m := range list {
			have := hc.byTeam[m.Team]
			if i := slices.IndexFunc(have, func(x proto.ChatMessage) bool { return x.ID == m.ID }); i >= 0 {
				have[i] = m // edited
				continue
			}
			hc.byTeam[m.Team] = append(have, m)
			if m.Account == me || (a.panelShows(panelChat) && a.chatTeam() == m.Team) {
				continue
			}
			if mentions(m, me) {
				a.notify(m.Name + " mentioned you in " + a.spaceName(m.Team) + ": " + firstLineOf(m.Text))
			} else {
				a.notify(m.Name + " in " + a.spaceName(m.Team) + ": " + firstLineOf(m.Text))
			}
		}
	}
}

// teamChatGoneEvent takes messages that were deleted.
func (a *App) teamChatGoneEvent(data json.RawMessage) func() {
	var refs []proto.ChatRef
	if json.Unmarshal(data, &refs) != nil {
		return nil
	}
	return func() {
		hc := a.chatsOf()
		for _, r := range refs {
			hc.byTeam[r.Team] = slices.DeleteFunc(hc.byTeam[r.Team], func(m proto.ChatMessage) bool { return m.ID == r.ID })
			if hc.editing != nil && hc.editing.ID == r.ID {
				a.stopEditing()
			}
		}
	}
}

// mentions says m mentions account.
func mentions(m proto.ChatMessage, account string) bool {
	return account != "" && slices.ContainsFunc(m.Mentions, func(mn proto.ChatMention) bool {
		return mn.Kind == proto.MentionUser && mn.ID == account
	})
}

func firstLineOf(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	if len(line) > 80 {
		line = line[:80] + "…"
	}
	return line
}

// chatUnread counts the messages of others since the user last read
// team's chat.
func (a *App) chatUnread(team string) int {
	if a.teamChats == nil || team == "" {
		return 0
	}
	read := a.state.ChatRead[team]
	n := 0
	for _, m := range a.teamChats.byTeam[team] {
		if m.ID > read && m.Account != a.cloudOf().st.AccountID {
			n++
		}
	}
	return n
}

// chatTab is the side panel's chat.
func (a *App) chatTab(c *ui.Context) {
	team := a.chatTeam()
	if team == "" {
		panelNote(c, "Sign in to enowX Cloud and join a team to chat with it.")
		return
	}
	hc := a.chatsOf()
	msgs, read := hc.byTeam[team]
	if !read {
		hc.byTeam[team] = nil
		a.loadChat(team)
	}
	if time.Since(hc.pulled) > chatRefresh {
		hc.pulled = time.Now()
		cl := a.client
		go func() { _ = cl.RefreshChat(team) }() // what came shows by its event
	}
	c.After(chatRefresh)
	if n := len(msgs); n > 0 {
		if a.state.ChatRead == nil {
			a.state.ChatRead = map[string]string{}
		}
		a.state.ChatRead[team] = msgs[n-1].ID
	}
	a.teamHeader(c, team)
	if len(msgs) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(6).Padding(24).Children(func() {
			ui.Icon(c, icon.Chat).FontSize(22).TextColor(theme.Idle)
			ui.Text(c, "No message yet: say hello to "+a.spaceName(team)+".").FontSize(12).TextColor(theme.Muted)
			ui.Text(c, "Type @ to mention a member, a note or a file.").FontSize(11).TextColor(theme.Idle)
		})
	} else {
		a.chatMessages(c, team, msgs)
	}
	a.chatComposer(c, team)
}

// teamHeader names the team the Team group's tools show, with how many
// members it has.
func (a *App) teamHeader(c *ui.Context, team string) {
	ui.Row(c).FillWidth().Height(36).Shrink(0).PaddingX(12).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		widget.TeamMark(c, team, a.spaceName(team), 16)
		ui.Text(c, a.spaceName(team)).FontSize(12.5).FontWeight(600).SingleLine().Shrink(1)
		if n := len(a.teamMembers(team)); n > 0 {
			ui.Text(c, textfmt.Plural(n, "1 member", fmt.Sprintf("%d members", n))).FontSize(11).TextColor(theme.Muted).SingleLine()
		}
		ui.Spacer(c)
		tip := "Sealed with the team's key: the cloud keeps only what it can't read"
		ui.Icon(c, icon.Lock).FontSize(12).TextColor(theme.Idle).Tooltip(tip).Label(tip)
	})
}
