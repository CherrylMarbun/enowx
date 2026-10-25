package ui

import (
	"net"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/ui/theme"
)

// dial serves a fresh daemon and connects to it.
func dial(t *testing.T) *client.Client {
	t.Helper()
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, func(proto.Event) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	return cl
}

// The notes of this device and of a server it watches end up on both,
// and Settings' Sync says so.
func TestNotesSyncWithAWatchedHost(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the syncs' bases go in this device's folder
	if _, err := transport.Dir(); err != nil {
		t.Skip(err)
	}
	hub, vps := dial(t), dial(t)
	mine, _ := hub.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Groceries"}, Body: "- [ ] milk\n"})
	theirs, _ := vps.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Server plan"}, Body: "- [ ] backups\n"})

	app := New(Options{})
	var later []func()
	app.onMain = func(fn func()) { later = append(later, fn) }
	app.client, app.host = hub, hosts.Host{ID: hosts.LocalID, Name: "This Mac"}
	vpsHost := hosts.Host{ID: "vps1", Name: "datalix-vps", Address: "root@vps"}
	app.hostList = []hosts.Host{vpsHost}
	app.watches = map[string]*hostWatch{"vps1": {host: vpsHost, cl: vps, status: hostStatus{state: hostOnline}}}
	app.page, app.settingsSection = pageSettings, sectionSync
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(900).Height(700).Background(theme.Background).Children(func() { app.settingsBody(c) })
	}, 900, 700)
	settle := func(what string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !done() {
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
			time.Sleep(10 * time.Millisecond)
			run := later
			later = nil
			for _, fn := range run {
				fn()
			}
			tt.Frame()
		}
	}
	app.syncData()
	settle("the sync", func() bool { return !app.dsync.running && app.dsync.hosts["vps1"].state == syncIdle })
	if n, err := vps.Note(mine.ID); err != nil || n.Body != "- [ ] milk\n" {
		t.Fatalf("this device's note on the server: %+v, %v", n, err)
	}
	if n, err := hub.Note(theirs.ID); err != nil || n.Title != "Server plan" {
		t.Fatalf("the server's note here: %+v, %v", n, err)
	}
	settle("the section", func() bool { return tt.HasText("Synced with 1 host(s)") && tt.HasText("datalix-vps") })

	// Changed on both, the same line: a conflict to settle, from Settings.
	for _, c := range []*client.Client{hub, vps} {
		n, _ := c.Note(theirs.ID)
		n.Body = "- [x] backups (" + map[*client.Client]string{hub: "mac", vps: "vps"}[c] + ")\n"
		c.SaveNote(n)
	}
	app.syncData()
	settle("the conflict", func() bool { return tt.HasText("Keep both") })
	shot(t, tt, "sync-settings.png")
	if err := tt.Click("Keep both"); err != nil {
		t.Fatal(err)
	}
	settle("the conflict settled", func() bool { return !app.dsync.running && len(app.dsync.conflicts) == 0 && !tt.HasText("Keep both") })
	list, _ := vps.Notes()
	if len(list) != 3 {
		t.Errorf("notes on the server after keeping both: %+v", list)
	}
}
