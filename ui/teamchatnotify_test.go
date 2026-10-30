package ui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A team member's new message shows a card to answer from, by the
// settings: every message, mentions only, none (mentions still); an old one a sync
// brought, an edit, or the user's own shows none.
func TestChatMessageNotifies(t *testing.T) {
	playSound = func([]byte, float64) error { return nil }
	app := New(Options{})
	app.onMain = func(fn func()) { fn() }
	msg := func(id, account, text string, at time.Time, mention bool) json.RawMessage {
		m := proto.ChatMessage{ID: id, Team: "t1", At: at, Account: account, Name: "Bob Lee", Text: text}
		if mention {
			m.Mentions = []proto.ChatMention{{Kind: "user", ID: "me", Title: "Ada"}}
		}
		data, _ := json.Marshal([]proto.ChatMessage{m})
		return data
	}
	app.cloudOf().st.AccountID = "me"
	now := time.Now()
	app.teamChatEvent(msg("1", "bob", "the canvas for the login page is ready, have a look when you can", now, false))()
	app.teamChatEvent(msg("2", "bob", "@Ada can you check the copy?", now, true))()
	app.teamChatEvent(msg("1", "bob", "edited", now, false))()                         // an edit
	app.teamChatEvent(msg("3", "me", "mine", now, false))()                            // the user's own
	app.teamChatEvent(msg("4", "bob", "from last week", now.Add(-time.Hour), false))() // a sync's
	if len(app.chatToasts) != 2 || !app.chatToasts[1].mention {
		t.Fatalf("cards: %d", len(app.chatToasts))
	}
	app.chatToasts[1].replying = true
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(900).Height(420).Background(theme.Background).TextColor(theme.Text).Children(func() { app.chatToastsView(c) })
	}, 900, 420)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Bob Lee") || !tt.HasText("@Ada can you check the copy?") {
		t.Fatalf("cards show: %q", tt.Texts())
	}
	shot(t, tt, "chat-toasts.png")

	app.chatToasts = nil
	app.ntf.settings, app.ntf.loaded = app.notifySettings(), true
	app.ntf.settings.Chat = "mentions"
	app.teamChatEvent(msg("5", "bob", "plain", now, false))()
	app.teamChatEvent(msg("6", "bob", "@Ada look", now, true))()
	if len(app.chatToasts) != 1 || app.chatToasts[0].msg.ID != "6" {
		t.Fatalf("mentions only: %d", len(app.chatToasts))
	}
	app.ntf.settings.Chat = "off"
	app.teamChatEvent(msg("7", "bob", "plain again", now, false))()
	if len(app.chatToasts) != 1 {
		t.Fatal("off still shows")
	}
	// Opening the chat takes its cards away.
	app.openChatAt(app.chatToasts[0].msg)
	if len(app.chatToasts) != 0 || app.state.Panel.Team != "t1" {
		t.Fatal("opening the chat")
	}
}

// A team set to off stays quiet, but a reply to the user notifies; the
// title bar counts what is unread.
func TestChatNotifiesPerTeam(t *testing.T) {
	playSound = func([]byte, float64) error { return nil }
	app := New(Options{})
	app.onMain = func(fn func()) { fn() }
	app.cloudOf().st.AccountID = "me"
	app.ntf.settings, app.ntf.loaded = app.notifySettings(), true
	app.ntf.settings.ChatTeams = map[string]string{"t1": "off"}
	send := func(m proto.ChatMessage) {
		data, _ := json.Marshal([]proto.ChatMessage{m})
		app.teamChatEvent(data)()
	}
	now := time.Now()
	send(proto.ChatMessage{ID: "1", Team: "t1", At: now, Account: "me", Name: "Ada", Text: "mine"})
	send(proto.ChatMessage{ID: "2", Team: "t1", At: now, Account: "bob", Name: "Bob", Text: "plain"})
	if len(app.chatToasts) != 0 {
		t.Fatal("an off team notified")
	}
	send(proto.ChatMessage{ID: "3", Team: "t1", At: now, Account: "bob", Name: "Bob", Text: "yes", ReplyTo: "1"})
	if len(app.chatToasts) != 1 {
		t.Fatal("a reply to the user didn't notify")
	}
	if n := app.chatUnread("t1"); n != 2 {
		t.Fatalf("unread %d", n)
	}
}
