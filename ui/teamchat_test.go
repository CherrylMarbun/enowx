//go:build !windows

package ui

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/hostauth"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// The side panel's Team group: the team's chat, a message typed in it
// mentioning the team's note with "@", sent with Enter, edited and
// deleted; the group's projects and members.
func TestTeamChatInTheSidePanel(t *testing.T) {
	dir := t.TempDir()
	const team = "0123456789abcdef"
	hostauth.Open(filepath.Join(dir, "cloud")).Put(hostauth.State{URL: "https://cloud.test", Token: "enxc_test", AccountID: "acc1", Account: "Ada"})
	os.MkdirAll(filepath.Join(dir, "teams", team), 0o700)
	os.WriteFile(filepath.Join(dir, "teams", team, "team.json"), []byte(`{"team":"`+team+`","name":"Design","role":"owner","account":"acc1"}`), 0o600)
	// Bob wrote first, twice in a row, mentioning Ada.
	chat := filepath.Join(dir, "teams", team, "chat")
	os.MkdirAll(chat, 0o700)
	at := time.Now().Add(-20 * time.Minute).UTC()
	os.WriteFile(filepath.Join(chat, "0000000000000000001-aaaa.json"), []byte(`{"at":"`+at.Format(time.RFC3339)+`","account":"acc2","name":"Bob","github":"bob","text":"morning @Ada, the canvas is ready","mentions":[{"kind":"user","id":"acc1","title":"Ada"}]}`), 0o600)
	os.WriteFile(filepath.Join(chat, "0000000000000000002-bbbb.json"), []byte(`{"at":"`+at.Add(time.Minute).Format(time.RFC3339)+`","account":"acc2","name":"Bob","github":"bob","text":"have a look when you can"}`), 0o600)
	srv, err := daemon.NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	app := New(Options{})
	var mu sync.Mutex
	var later []func()
	app.onMain = func(fn func()) { mu.Lock(); later = append(later, fn); mu.Unlock() }
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, func(ev proto.Event) { app.handleEvent(1, ev) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	app.gen, app.client = 1, cl
	app.host = hosts.Host{ID: hosts.LocalID, Name: "this Mac"}
	app.state.Space = team
	cl.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Roadmap", Team: team}})
	app.state.Panel.Open, app.state.Panel.Tab = true, panelChat
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(460).Height(640).Background(theme.Background).TextColor(theme.Text).Children(func() {
			app.panelStrip(c)
			switch app.panelTab() {
			case panelTeamProjects:
				app.teamProjectsTab(c)
			case panelTeamMembers:
				app.teamMembersTab(c)
			default:
				app.chatTab(c)
			}
		})
	}, 460, 640)
	settle := func(what string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			tt.Frame()
			if done() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			fns := later
			later = nil
			mu.Unlock()
			for _, fn := range fns {
				fn()
			}
		}
	}
	settle("the team's chat", func() bool { return tt.HasText("Team") && tt.HasText("have a look when you can") })
	if !tt.HasText("Bob") || !tt.HasText("@Ada") {
		t.Fatalf("Bob's messages, under his name, mentioning Ada: %q", tt.Texts())
	}
	app.notesOf()
	settle("the team's note to mention", func() bool { return len(app.notesOf().list) == 1 })
	// "@" and a few letters offer the note; Enter picks it, then sends.
	if err := tt.Click("Mention (@)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("road")
	settle("the mention menu", func() bool { return tt.HasText("Roadmap") && tt.HasText("MENTION") })
	shot(t, tt, "team-chat-mention.png")
	tt.Key(0, ui.KeyEnter)
	settle("the note mentioned", func() bool { return app.chatsOf().draft == "@Roadmap " })
	tt.Type("is up")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	settle("the message", func() bool { return tt.HasText("is up") && len(app.chatsOf().byTeam[team]) == 3 })
	if m := app.chatsOf().byTeam[team][2]; m.Text != "@Roadmap is up" || len(m.Mentions) != 1 || m.Mentions[0].Title != "Roadmap" {
		t.Fatalf("the message: %+v", m)
	}
	if app.chatsOf().draft != "" {
		t.Fatalf("the draft after sending: %q", app.chatsOf().draft)
	}
	// Up in the empty composer edits the last message; hovering it offers
	// to delete it, once confirmed.
	tt.Key(0, ui.KeyUp)
	settle("editing", func() bool { return tt.HasText("Editing your message · Esc to cancel") })
	tt.Type(" now")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	settle("the edit", func() bool {
		m := app.chatsOf().byTeam[team]
		return len(m) == 3 && m[2].Text == "@Roadmap is up now" && m[2].Edited != nil && tt.HasText("  (edited)")
	})
	shot(t, tt, "team-chat.png")
	r, ok := tt.Find("@Roadmap is up now  (edited)")
	if !ok {
		t.Fatal("the edited message isn't shown")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	shot(t, tt, "team-chat-hover.png")
	if err := tt.Click("Delete"); err != nil {
		t.Fatalf("no Delete on hover: %v; shows %q", err, tt.Texts())
	}
	if app.confirming == nil {
		t.Fatal("deleted without asking")
	}
	app.confirming.do()
	settle("the message gone", func() bool { return len(app.chatsOf().byTeam[team]) == 2 && !tt.HasText("is up now") })

	// A message whose mention shows as a chip, not in its text, keeps it
	// through an edit.
	if _, err := cl.SendChat(proto.ChatSendParams{Team: team, Text: "ni bagus", Mentions: []proto.ChatMention{{Kind: proto.MentionCanvas, ID: "c1", Title: "Untitled canvas"}}}); err != nil {
		t.Fatal(err)
	}
	settle("the message with a chip", func() bool { return len(app.chatsOf().byTeam[team]) == 3 })
	app.startEditing(app.chatsOf().byTeam[team][2])
	app.chatsOf().draft = "ni bagus banget"
	app.sendChat(team, app.chatsOf().draft)
	settle("the edit keeping its chip", func() bool {
		m := app.chatsOf().byTeam[team][2]
		return m.Text == "ni bagus banget" && len(m.Mentions) == 1 && m.Mentions[0].Title == "Untitled canvas"
	})
	// The group's other tools: the team's projects, its members.
	if err := tt.Click("Projects"); err != nil {
		t.Fatal(err)
	}
	settle("the projects", func() bool { return tt.HasText("No projects yet") })
	if err := tt.Click("Members"); err != nil {
		t.Fatal(err)
	}
	settle("the members, the cloud out of reach", func() bool { return tt.HasText("Try again") })
}
