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

// The side panel's Team group: the team's chat, a message sent from it
// listed, the team's note mentioned in it.
func TestTeamChatInTheSidePanel(t *testing.T) {
	dir := t.TempDir()
	const team = "0123456789abcdef"
	hostauth.Open(filepath.Join(dir, "cloud")).Put(hostauth.State{URL: "https://cloud.test", Token: "enxc_test", AccountID: "acc1", Account: "Ada"})
	os.MkdirAll(filepath.Join(dir, "teams", team), 0o700)
	os.WriteFile(filepath.Join(dir, "teams", team, "team.json"), []byte(`{"team":"`+team+`","name":"Design","role":"owner","account":"acc1"}`), 0o600)
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
			app.chatTab(c)
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
	settle("the team's chat", func() bool { return tt.HasText("Team") && tt.HasText("No message yet: say hello to Design.") })
	app.notesOf()
	settle("the team's note to mention", func() bool { return len(app.notesOf().list) == 1 })
	if err := tt.Click("Mention"); err != nil {
		t.Fatal(err)
	}
	settle("the mention menu", func() bool { return tt.HasText("Roadmap") })
	if err := tt.Click("Roadmap"); err != nil {
		t.Fatal(err)
	}
	app.chatsOf().draft = "the roadmap is up"
	tt.Frame()
	if err := tt.Click("Send"); err != nil {
		t.Fatal(err)
	}
	settle("the message", func() bool { return tt.HasText("the roadmap is up") && len(app.chatsOf().byTeam[team]) == 1 })
	if m := app.chatsOf().byTeam[team][0]; len(m.Mentions) != 1 || m.Mentions[0].Title != "Roadmap" {
		t.Fatalf("its mention: %+v", m)
	}
	shot(t, tt, "team-chat.png")
}
