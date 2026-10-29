package ui

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// A team's chat (docs/cloud.md), the side panel's Team group: the team of
// the selected session's project, else the space shown. Its messages come
// through the team's space, read again every few seconds while it shows;
// one may mention the team's notes, canvases and projects.

// panelChat is the side panel's chat tool.
const panelChat = "chat"

// chatRefresh is how often an open chat takes what the team sent.
const chatRefresh = 5 * time.Second

// hostChats is the connected host's teams' chats, as last read. It
// belongs to the main thread.
type hostChats struct {
	host     string
	byTeam   map[string][]proto.ChatMessage
	list     ui.ListState // the messages, following the latest
	draft    string
	mentions []proto.ChatMention
	pulled   time.Time
	err      string
}

func (a *App) chatsOf() *hostChats {
	if a.teamChats == nil || a.teamChats.host != a.host.ID {
		a.teamChats = &hostChats{host: a.host.ID, byTeam: map[string][]proto.ChatMessage{}}
	}
	return a.teamChats
}

// chatTeam is the team whose chat the panel shows.
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

// teamChatEvent takes messages that came or were sent.
func (a *App) teamChatEvent(data json.RawMessage) func() {
	var list []proto.ChatMessage
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		hc := a.chatsOf()
		for _, m := range list {
			have := hc.byTeam[m.Team]
			if slices.ContainsFunc(have, func(x proto.ChatMessage) bool { return x.ID == m.ID }) {
				continue
			}
			hc.byTeam[m.Team] = append(have, m)
			if m.Account != a.cloudOf().st.AccountID && !(a.panelShows(panelChat) && a.chatTeam() == m.Team) {
				a.notify(m.Name + " in " + a.spaceName(m.Team) + ": " + firstLineOf(m.Text))
			}
		}
	}
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
	ui.Row(c).FillWidth().Height(36).Shrink(0).PaddingX(12).Gap(8).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		widget.TeamMark(c, team, a.spaceName(team), 16)
		ui.Text(c, a.spaceName(team)).FontSize(12.5).SingleLine()
		ui.Text(c, "· sealed with the team's key").FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1)
	})
	if len(msgs) == 0 {
		ui.Column(c).Grow(1).FillWidth().Padding(12).Children(func() {
			ui.Text(c, "No message yet: say hello to "+a.spaceName(team)+".").FontSize(12).TextColor(theme.Muted)
		})
	} else {
		now := time.Now()
		hc.list.FollowEnd = true
		ui.List(c, &hc.list, len(msgs), func(i int) {
			ui.Column(c).FillWidth().Padding(6, 12).Children(func() { a.chatMessage(c.Key("m:"+msgs[i].ID), msgs[i], now) })
		}).Grow(1).FillWidth().MinHeight(0)
	}
	if hc.err != "" {
		widget.Notice(c, icon.Alert, theme.Error, hc.err, "", nil)
	}
	a.chatComposer(c, team)
}

func (a *App) chatMessage(c *ui.Context, m proto.ChatMessage, now time.Time) {
	ui.Column(c).FillWidth().Gap(3).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			who := m.Name
			if m.GitHub != "" {
				who += " · @" + m.GitHub
			}
			ui.Text(c, who).FontSize(12).FontWeight(600).SingleLine()
			ui.Text(c, textfmt.Age(m.At, now)).FontSize(10.5).TextColor(theme.Muted).Tooltip(m.At.Local().Format("2 Jan 15:04"))
		})
		ui.Text(c, m.Text).FontSize(12.5).TextColor(theme.Text).Selectable()
		if len(m.Mentions) > 0 {
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, mn := range m.Mentions {
					a.mentionChip(c.Key(mn.Kind+":"+mn.ID), m.Team, mn)
				}
			})
		}
	})
}

// mentionChip is an item a message names; a note or canvas opens.
func (a *App) mentionChip(c *ui.Context, team string, mn proto.ChatMention) {
	svg := map[string]*ui.SVG{proto.MentionNote: icon.Notebook, proto.MentionCanvas: icon.Canvas, proto.MentionProject: icon.Folder, proto.MentionSession: icon.Terminal}[mn.Kind]
	if svg == nil {
		svg = icon.Link
	}
	b := ui.ButtonBase(c).Height(22).Padding(0, 8, 0, 6).Gap(5).Radius(11).Background(theme.Sunken).Border(1, theme.Border).AlignItems(ui.Center).
		Label(mn.Kind + " " + mn.Title)
	if mn.Kind == proto.MentionNote || mn.Kind == proto.MentionCanvas {
		b.Cursor(ui.CursorPointer)
	}
	b.Children(func() {
		ui.Icon(c, svg).FontSize(11).TextColor(theme.TextSoft)
		ui.Text(c, mn.Title).FontSize(11).TextColor(theme.TextSoft).SingleLine()
	})
	if b.Clicked() {
		switch mn.Kind {
		case proto.MentionNote:
			a.openNote(mn.ID, team)
		case proto.MentionCanvas:
			a.showSpace(team)
			a.page = pageCanvas
			if hc := a.canvasesOf(); hc != nil {
				hc.open = mn.ID
			}
		}
	}
}

// chatComposer writes and sends a message, with the items it mentions.
func (a *App) chatComposer(c *ui.Context, team string) {
	hc := a.chatsOf()
	ui.Column(c).FillWidth().Shrink(0).Padding(10, 12).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(theme.Border).Children(func() {
		if len(hc.mentions) > 0 {
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for i, mn := range hc.mentions {
					a.mentionChip(c.Key("draft:"+mn.ID), team, mn)
					if widget.IconButton(c.Key("drop:"+mn.ID), icon.Close, "Don't mention "+mn.Title, false).Clicked() {
						hc.mentions = slices.Delete(slices.Clone(hc.mentions), i, i+1)
					}
				}
			})
		}
		ui.TextArea(c, &hc.draft).Height(60).Padding(8, 10).Radius(theme.RadiusControl).FontSize(12.5).
			Background(theme.Sunken).Border(1, theme.BorderStrong).Placeholder("Message " + a.spaceName(team))
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			more := widget.Button(c, widget.Secondary, icon.Link, "Mention", "")
			a.menu.OpenBelow(more, "chat-mention", 24)
			a.menu.Show(c, more, "chat-mention", func(done func()) { a.mentionMenu(c, team, done) })
			ui.Spacer(c)
			send := widget.PrimaryButton(c, icon.Send, "Send", "")
			if strings.TrimSpace(hc.draft) == "" {
				send.Disabled(true)
			} else if send.Clicked() {
				p := proto.ChatSendParams{Team: team, Text: hc.draft, Mentions: hc.mentions}
				cl := a.client
				go func() {
					_, err := cl.SendChat(p)
					a.onMain(func() {
						hc.err = ""
						if err != nil {
							hc.err = strings.TrimPrefix(client.Explain(err), "teams: ")
							return
						}
						hc.draft, hc.mentions = "", nil
					})
				}()
			}
		})
	})
}

// mentionMenu lists the team's notes, canvases and projects to mention.
func (a *App) mentionMenu(c *ui.Context, team string, done func()) {
	hc := a.chatsOf()
	add := func(mn proto.ChatMention) {
		if !slices.ContainsFunc(hc.mentions, func(x proto.ChatMention) bool { return x.Kind == mn.Kind && x.ID == mn.ID }) {
			hc.mentions = append(hc.mentions, mn)
		}
		done()
	}
	any := false
	for _, n := range a.notesOf().list {
		if n.Team == team && n.Parent == "" {
			any = true
			if widget.MenuItem(c, icon.Notebook, noteTitle(n), false) {
				add(proto.ChatMention{Kind: proto.MentionNote, ID: n.ID, Title: noteTitle(n)})
			}
		}
	}
	for _, cv := range a.canvasesOf().list {
		if cv.Team == team {
			any = true
			if widget.MenuItem(c, icon.Canvas, cv.Title, false) {
				add(proto.ChatMention{Kind: proto.MentionCanvas, ID: cv.ID, Title: cv.Title})
			}
		}
	}
	for _, p := range a.teamProjectsOf().list {
		if p.Team == team {
			any = true
			if widget.MenuItem(c, icon.Folder, p.Name, false) {
				add(proto.ChatMention{Kind: proto.MentionProject, ID: p.Ref, Title: p.Name})
			}
		}
	}
	if !any {
		widget.MenuItem(c, icon.Info, "The team has no notes, canvases or projects yet", false)
	}
}
