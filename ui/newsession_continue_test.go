package ui

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// New session's first field lists the project's sessions and its agents'
// past conversations: picking a running one opens it and sends it the
// message; picking a past conversation resumes it.
func TestNewSessionGoesOnWithASessionOfTheProject(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	pa, pb := net.Pipe()
	go srv.ServeConn(pa)
	cl, err := client.New(pb, func(proto.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	dir := t.TempDir()
	if _, err := cl.AddProject(dir); err != nil {
		t.Fatal(err)
	}
	sh, err := cl.CreateSession(proto.SessionCreateParams{Project: dir, Agent: "shell", Name: "build shell", Cols: 80, Rows: 20})
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	var later []func()
	app.onMain = func(fn func()) { later = append(later, fn) }
	app.client, app.host = cl, hosts.Host{ID: hosts.LocalID, Name: "This Mac"}
	app.agents = []proto.AgentInfo{{ID: "shell", Name: "Shell", Installed: true}, {ID: "claude", Name: "Claude Code", Command: "claude", Installed: true}}
	app.sessions = []proto.SessionInfo{sh}
	app.openNewSession(proto.ProjectInfo{Name: "proj", Path: dir})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).FillWidth().FillHeight().Background(theme.Background).Children(func() { app.newSessionDialog(c) })
	}, 1000, 900)
	settle := func(what string, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(8 * time.Second); ; {
			tt.Frame()
			fns := later
			later = nil
			for _, fn := range fns {
				fn()
			}
			if done() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatalf("%v; shows %q", err, tt.Texts())
		}
		tt.Frame()
	}
	settle("the dialog", func() bool { return tt.HasText("Session") })
	quiet := time.Now().Add(500 * time.Millisecond) // the host's conversations read
	settle("a moment", func() bool { return time.Now().After(quiet) })
	app.newSession.convs = []proto.Conversation{{Agent: "claude", ID: "conv-9", Title: "Fix the login bug", Dir: dir, Updated: time.Now().Add(-time.Hour)}}
	tt.Frame()
	click("Session: New session")
	settle("the list", func() bool { return tt.HasText("Fix the login bug") && tt.HasText("build shell") })
	shot(t, tt, "new-session-continue.png")

	click("build shell")
	settle("open it", func() bool { return tt.HasText("Open session") })
	m, _ := tt.Find("Message (optional)")
	tt.ClickAt(m.X+10, m.Y+35)
	tt.Type("echo picked-from-new-session")
	click("Open session")
	settle("opened", func() bool { return !app.newSession.open && app.selected == sh.ID })
	var screen string
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline) && !strings.Contains(screen, "picked-from-new-session"); time.Sleep(50 * time.Millisecond) {
		screen, _ = cl.Screen(sh.ID)
	}
	if !strings.Contains(screen, "picked-from-new-session") {
		t.Fatalf("the message isn't in the session: %q", screen)
	}

	app.openNewSession(proto.ProjectInfo{Name: "proj", Path: dir})
	settle("the dialog again", func() bool { return tt.HasText("Session") })
	quiet = time.Now().Add(500 * time.Millisecond)
	settle("a moment", func() bool { return time.Now().After(quiet) })
	app.newSession.convs = []proto.Conversation{{Agent: "claude", ID: "conv-9", Title: "Fix the login bug", Dir: dir, Updated: time.Now()}}
	tt.Frame()
	click("Session: New session")
	settle("the list", func() bool { return tt.HasText("Fix the login bug") })
	click("Fix the login bug")
	settle("resume it", func() bool { return tt.HasText("Resume") })
	click("Resume")
	settle("resumed", func() bool { return !app.newSession.open })
	list, _ := cl.Sessions()
	found := false
	for _, s := range list {
		found = found || s.Agent == "claude" && s.ID != sh.ID
	}
	if !found {
		t.Fatalf("no claude session resumed: %+v", list)
	}
}
