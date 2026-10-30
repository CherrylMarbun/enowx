package ui

import (
	"net"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A session renamed from its menu takes the name on the host, for every
// app of it; renamed empty, it goes back to the name it takes by itself.
func TestRenameASession(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a1, b1 := net.Pipe()
	go srv.ServeConn(a1)
	cl, err := client.New(b1, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s, err := cl.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	app.onMain = func(fn func()) { fn() }
	app.client = cl
	app.sessions = []proto.SessionInfo{s}
	menu := true
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(theme.Background).TextColor(theme.Text).Children(func() {
			if menu {
				app.sessionMenuItems(c, app.sessions[0], func() { menu = false })
			}
			app.renameDialog(c)
		})
	}, 900, 700)
	tt.Frame()
	if err := tt.Click("Rename…"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	tt.Frame()
	if app.renaming == nil || !tt.HasText("Rename session") {
		t.Fatalf("no dialog: %q", tt.Texts())
	}
	named := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			list, err := cl.Sessions()
			if err == nil && len(list) == 1 && list[0].Name == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("the host's name: %+v, want %q", list, want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	app.renaming.name = "  Checkout   redesign "
	tt.Frame()
	if err := tt.Click("Rename"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	named("Checkout redesign")
	if app.renaming != nil || app.sessionName(app.sessions[0]) != "Checkout redesign" {
		t.Fatalf("shown: %q", app.sessionName(app.sessions[0]))
	}
	app.askRename(app.sessions[0])
	app.renaming.name = ""
	tt.Frame()
	if err := tt.Click("Rename"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	named("")
}
