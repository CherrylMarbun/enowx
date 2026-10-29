package ui

import (
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// The chat's composer: one line growing with what is typed, Enter sending
// and Shift+Enter breaking the line. "@" opens a menu of what to mention,
// narrowed as more is typed: the team's members, notes, canvases and
// projects, and the files of the selected session when it works in one
// of the team's projects. Picking one writes "@Title" in the message.

// mentionShown is how many options the "@" menu shows at most.
const mentionShown = 8

// mentionOption is what an "@" may mention.
type mentionOption struct {
	m      proto.ChatMention
	detail string
}

func (a *App) chatComposer(c *ui.Context, team string) {
	hc := a.chatsOf()
	ui.Column(c).FillWidth().Shrink(0).Padding(8, 10, 10, 10).Gap(6).BorderWidth(1, 0, 0, 0).BorderColor(theme.Border).Children(func() {
		if hc.err != "" {
			widget.Notice(c, icon.Alert, theme.Error, hc.err, "", nil)
		}
		if hc.editing != nil {
			ui.Row(c).FillWidth().Gap(6).PaddingX(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icon.Pencil).FontSize(12).TextColor(theme.Accent)
				ui.Text(c, "Editing your message · Esc to cancel").FontSize(11.5).TextColor(theme.TextSoft).Grow(1).Shrink(1).SingleLine()
				if widget.IconButton(c, icon.Close, "Stop editing", false).Clicked() {
					a.stopEditing()
				}
			})
			// What the message mentions as chips, kept unless dropped here.
			ui.Row(c).FillWidth().Gap(6).Wrap().PaddingX(4).Children(func() {
				for i, mn := range hc.mentions {
					if strings.Contains(hc.editing.Text, "@"+mn.Title) {
						continue
					}
					a.mentionChip(c.Key("keep:"+mn.Kind+":"+mn.ID), team, mn)
					if widget.IconButton(c.Key("drop:"+mn.Kind+":"+mn.ID), icon.Close, "Don't mention "+mn.Title, false).Clicked() {
						hc.mentions = slices.Delete(slices.Clone(hc.mentions), i, i+1)
					}
				}
			})
		}
		a.replyBar(c)
		start, stop, query, typing := -1, -1, "", false
		var options []mentionOption
		box := ui.Row(c).FillWidth().Gap(4).Padding(3, 3, 3, 10).Radius(14).Background(theme.Sunken).AlignItems(ui.End)
		border := theme.BorderStrong
		if box.FocusWithin() {
			border = theme.AccentLine
		}
		send := false
		box.Border(1, widget.Tint(box, "border", border)).Children(func() {
			in := ui.TextAreaBase(c, &hc.draft).Grow(1).Shrink(1).MinWidth(0).Lines(1, 6).Padding(6, 0).FontSize(12.5).
				Placeholder("Message " + a.spaceName(team) + " · @ to mention").Label("Message").
				Tooltip("Enter to send · Shift+Enter for a new line")
			_, caret := in.TextSelection()
			if hc.setCaret >= 0 {
				in.SetTextSelection(hc.setCaret, hc.setCaret)
				caret, hc.setCaret = hc.setCaret, -1
			}
			if in.Focused() && hc.draft != hc.dismissed {
				start, stop, query, typing = mentionQueryAt(hc.draft, caret)
			}
			if typing {
				options = a.mentionOptions(team, query)
				hc.pick = min(hc.pick, max(len(options)-1, 0))
			} else {
				hc.pick = 0
			}
			composing := in.Composing()
			in.OnTextKey(func(k ui.TextKey) bool {
				if k.Mods != 0 || composing {
					return false // Shift+Enter breaks the line
				}
				if typing && len(options) > 0 {
					switch k.Key {
					case ui.KeyDown:
						hc.pick = (hc.pick + 1) % len(options)
						return true
					case ui.KeyUp:
						hc.pick = (hc.pick + len(options) - 1) % len(options)
						return true
					case ui.KeyTab, ui.KeyEnter:
						value := a.pickMention(k.Text(), start, stop, options[hc.pick].m)
						k.SetText(value)
						return true
					case ui.KeyEscape:
						hc.dismissed = k.Text()
						return true
					}
				}
				switch {
				case k.Key == ui.KeyEscape && hc.editing != nil:
					a.stopEditing()
					k.SetText(hc.draft)
					return true
				case k.Key == ui.KeyEscape && hc.replyTo != nil:
					hc.replyTo = nil
					return true
				case k.Key == ui.KeyUp && k.Text() == "" && hc.editing == nil:
					// Up in an empty composer edits the user's last message.
					if m, ok := a.lastOwnMessage(team); ok {
						a.startEditing(m)
						k.SetText(hc.draft)
						return true
					}
					return false
				case k.Key != ui.KeyEnter:
					return false
				}
				if a.sendChat(team, k.Text()) {
					k.SetText(hc.draft)
				}
				return true
			})
			if hc.focus {
				in.Focus()
				hc.focus = false
			}
			if widget.IconButton(c, icon.AtSign, "Mention (@)", false).Clicked() {
				if hc.draft != "" && !strings.HasSuffix(hc.draft, " ") && !strings.HasSuffix(hc.draft, "\n") {
					hc.draft += " "
				}
				hc.draft += "@"
				hc.setCaret, hc.dismissed, hc.focus = len([]rune(hc.draft)), "", true
			}
			empty := strings.TrimSpace(hc.draft) == ""
			b := ui.ButtonBase(c).Size(28, 28).Radius(14).Center().Shrink(0).Label("Send").Tooltip("Send (Enter)").Cursor(ui.CursorPointer).
				Gradient(theme.AccentHigh, theme.Accent, 180).TextColor(theme.OnAccent)
			if empty {
				b.Opacity(0.35).Disabled(true)
			}
			b.Children(func() { ui.Icon(c, icon.ArrowUp).FontSize(13) })
			send = widget.Pressable(b).Clicked() && !empty
		})
		if typing {
			a.mentionMenu(c, box, start, stop, query, options)
		}
		if send && a.sendChat(team, hc.draft) {
			hc.focus = true
		}
	})
}

// sendChat sends text to team's chat with the mentions it still names,
// or saves it over the message being edited, and empties the draft (the
// one put aside, after an edit); false when there's nothing to send.
func (a *App) sendChat(team, text string) bool {
	hc, cl := a.chatsOf(), a.client
	if strings.TrimSpace(text) == "" || cl == nil {
		return false
	}
	editing := hc.editing
	var mentions []proto.ChatMention
	for _, mn := range hc.mentions {
		// A mention the message held as a chip, not written in its text,
		// stays through an edit: only "@Title" taken out drops one.
		chip := editing != nil && !strings.Contains(editing.Text, "@"+mn.Title)
		if (chip || strings.Contains(text, "@"+mn.Title)) && !slices.ContainsFunc(mentions, func(x proto.ChatMention) bool { return x.Kind == mn.Kind && x.ID == mn.ID }) {
			mentions = append(mentions, mn)
		}
	}
	replyTo := ""
	if hc.replyTo != nil && editing == nil {
		replyTo = hc.replyTo.ID
	}
	hc.draft, hc.mentions, hc.editing, hc.replyTo = "", nil, nil, nil
	a.stoppedTyping(team)
	if editing != nil {
		hc.draft, hc.saved = hc.saved, ""
	}
	go func() {
		var err error
		if editing != nil {
			_, err = cl.EditChat(proto.ChatEditParams{Team: team, ID: editing.ID, Text: text, Mentions: mentions})
		} else {
			_, err = cl.SendChat(proto.ChatSendParams{Team: team, Text: text, Mentions: mentions, ReplyTo: replyTo})
		}
		a.onMain(func() {
			hc.err = ""
			if err != nil {
				hc.err = strings.TrimPrefix(client.Explain(err), "teams: ")
				if editing == nil {
					hc.draft, hc.mentions = text, mentions // to send again
				}
			}
		})
	}()
	return true
}

// startEditing puts the user's message m in the composer to change it,
// the draft put aside meanwhile.
func (a *App) startEditing(m proto.ChatMessage) {
	hc := a.chatsOf()
	if hc.editing == nil {
		hc.saved = hc.draft
	}
	hc.editing = &m
	hc.draft, hc.mentions = m.Text, slices.Clone(m.Mentions)
	hc.setCaret, hc.focus, hc.dismissed = len([]rune(m.Text)), true, m.Text
}

// stopEditing leaves the message as it was and gives the draft back.
func (a *App) stopEditing() {
	hc := a.chatsOf()
	if hc.editing == nil {
		return
	}
	hc.editing, hc.mentions = nil, nil
	hc.draft, hc.saved = hc.saved, ""
	hc.setCaret, hc.focus = len([]rune(hc.draft)), true
}

// lastOwnMessage is the user's last message in team's chat.
func (a *App) lastOwnMessage(team string) (proto.ChatMessage, bool) {
	me, msgs := a.cloudOf().st.AccountID, a.chatsOf().byTeam[team]
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Account == me && me != "" {
			return msgs[i], true
		}
	}
	return proto.ChatMessage{}, false
}

// pickMention writes mn over the "@query" from start to stop of text,
// keeps it for sending and returns the new text, the caret after it.
func (a *App) pickMention(text string, start, stop int, mn proto.ChatMention) string {
	hc := a.chatsOf()
	runes := []rune(text)
	put := "@" + mn.Title + " "
	value := string(runes[:start]) + put + string(runes[stop:])
	hc.mentions = append(hc.mentions, mn)
	hc.setCaret, hc.pick, hc.focus = start+len([]rune(put)), 0, true
	return value
}

// mentionQueryAt finds the "@" being typed before caret, a rune offset of
// text: from it to the caret, and what follows it, lowercased.
func mentionQueryAt(text string, caret int) (start, stop int, query string, ok bool) {
	runes := []rune(text)
	if caret < 0 || caret > len(runes) {
		return
	}
	for at := caret - 1; at >= 0; at-- {
		r := runes[at]
		if r == '@' {
			if at > 0 && !unicode.IsSpace(runes[at-1]) && !strings.ContainsRune("([{", runes[at-1]) {
				return // an address, as ada@example.com
			}
			return at, caret, strings.ToLower(string(runes[at+1 : caret])), true
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("-_./", r) {
			return
		}
	}
	return
}

// mentionOptions are what team's "@query" may mention, best first: those
// starting with it, then those holding it.
func (a *App) mentionOptions(team, query string) []mentionOption {
	var all []mentionOption
	me := a.cloudOf().st.AccountID
	for _, m := range a.teamMembers(team) {
		if m.Account.ID == me {
			continue
		}
		detail := m.Role
		if m.Account.GitHub != "" {
			detail = "@" + m.Account.GitHub + " · " + m.Role
		}
		all = append(all, mentionOption{proto.ChatMention{Kind: proto.MentionUser, ID: m.Account.ID, Title: memberTitle(m.Account.Name, m.Account.GitHub)}, detail})
	}
	for _, n := range a.notesOf().list {
		if n.Team == team && n.Parent == "" {
			all = append(all, mentionOption{proto.ChatMention{Kind: proto.MentionNote, ID: n.ID, Title: noteTitle(n)}, "Note"})
		}
	}
	for _, cv := range a.canvasesOf().list {
		if cv.Team == team {
			all = append(all, mentionOption{proto.ChatMention{Kind: proto.MentionCanvas, ID: cv.ID, Title: cv.Title}, "Canvas"})
		}
	}
	for _, p := range a.teamProjectsOf().list {
		if p.Team == team {
			all = append(all, mentionOption{proto.ChatMention{Kind: proto.MentionProject, ID: p.Ref, Title: p.Name}, "Project"})
		}
	}
	all = append(all, a.mentionFiles(team, query)...)
	var first, then []mentionOption
	for _, o := range all {
		title := strings.ToLower(o.m.Title)
		switch {
		case strings.HasPrefix(title, query) || (o.m.Kind == proto.MentionUser && strings.Contains(strings.ToLower(o.detail), "@"+query)):
			first = append(first, o)
		case strings.Contains(title, query) || (o.m.Kind == proto.MentionFile && strings.Contains(strings.ToLower(o.detail), query)):
			then = append(then, o)
		}
	}
	out := append(first, then...)
	if len(out) > mentionShown {
		out = out[:mentionShown]
	}
	return out
}

// memberTitle is how a member is mentioned: their name, else their login.
func memberTitle(name, github string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return github
}

// mentionFiles are the files of the selected session's folder matching
// query, when it works in one of team's projects: found on the host, the
// menu showing them once they come.
func (a *App) mentionFiles(team, query string) []mentionOption {
	s, ok := a.selectedSession()
	if !ok || query == "" || a.client == nil {
		return nil
	}
	p, ok := a.teamOfProject(s.Project)
	if !ok || p.Team != team {
		return nil
	}
	hc, cl := a.chatsOf(), a.client
	if hc.filesAsked != s.ID+"\x00"+query {
		hc.filesAsked = s.ID + "\x00" + query
		asked := hc.filesAsked
		go func() {
			paths, err := cl.FindFiles(s.ID, query, 5)
			a.onMain(func() {
				if err == nil && hc.filesAsked == asked {
					hc.files, hc.filesFor = paths, asked
				}
			})
		}()
	}
	if hc.filesFor != hc.filesAsked {
		return nil
	}
	var out []mentionOption
	for _, f := range hc.files {
		out = append(out, mentionOption{proto.ChatMention{Kind: proto.MentionFile, ID: f, Title: path.Base(f), Project: p.Ref}, f})
	}
	return out
}

// mentionMenu lists the options over the composer box; arrows move,
// Tab or Enter picks, Escape closes it.
func (a *App) mentionMenu(c *ui.Context, box ui.Element, start, stop int, query string, options []mentionOption) {
	hc := a.chatsOf()
	ui.Overlay(c, func() {
		widget.Popup(ui.Column(c).AttachTo(box, ui.AnchorTopLeft, ui.AnchorBottomLeft).Margin(0, 0, 6, 0).
			Width(max(box.LayoutBounds().W, 240)).MaxHeight(300)).Children(func() {
			if len(options) == 0 {
				msg := "Nothing in the team is called “" + query + "”"
				if query == "" {
					msg = "The team has no members, notes or projects to mention yet"
				}
				ui.Text(c, msg).FontSize(12).TextColor(theme.Muted).Padding(8, 10)
				return
			}
			widget.PopupLabel(c, "MENTION")
			for i, o := range options {
				row := ui.ButtonBase(c.Key(o.m.Kind + ":" + o.m.ID)).Label(o.m.Title).Cursor(ui.CursorPointer)
				widget.PopupItem(row, i == hc.pick || row.Hovered()).Children(func() {
					if o.m.Kind == proto.MentionUser {
						memberAvatar(c, o.m.ID, o.m.Title, 18)
					} else {
						ui.Box(c).Size(18, 18).Center().Shrink(0).Children(func() {
							ui.Icon(c, mentionIcon(o.m.Kind)).FontSize(13).TextColor(theme.TextSoft)
						})
					}
					ui.Text(c, o.m.Title).FontSize(12.5).TextColor(theme.Text).SingleLine().Shrink(0).MaxWidth(200)
					ui.Text(c, o.detail).FontSize(11).TextColor(theme.Muted).SingleLine().Shrink(1).Grow(1)
				})
				if row.Clicked() {
					hc.draft = a.pickMention(hc.draft, start, stop, o.m)
				}
			}
		})
	})
}
