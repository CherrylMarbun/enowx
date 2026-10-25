package ui

import (
	"net"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/notify"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A project's copies changed on both hosts: the user hears it once, the
// project shows how many files wait, and its mark opens where to pick.
func TestLinkConflictsSoundAndMark(t *testing.T) {
	snd := catchSounds(t)
	app := New(Options{})
	app.onMain = func(fn func()) { fn() }
	app.ntf.loaded = true
	app.ntf.settings = notify.Defaults()
	app.daemonInfo.ID = "mac"
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
	app.client = cl
	p := proto.ProjectInfo{Name: "shop", Path: "/w/shop"}
	app.projects = []proto.ProjectInfo{p}
	l := proto.ProjectLink{ID: "l1", Name: "shop", Source: proto.ProjectEnd{Daemon: "mac", Path: p.Path, Host: "Mac"},
		Target: proto.ProjectEnd{Daemon: "vps", Path: "/srv/shop", Host: "VPS"}}
	conflicts := []proto.ProjectSyncConflict{{Path: "a.go"}, {Path: "b.go"}}
	app.links.links = map[string]proto.ProjectLinkState{l.ID: {ProjectLink: l, Status: proto.ProjectLinkStatus{Conflicts: conflicts}}}

	app.tellConflicts(l, conflicts)
	app.tellConflicts(l, conflicts) // the same two again: no new sound
	time.Sleep(soundGather + 300*time.Millisecond)
	if got := snd.take(); len(got) != 1 {
		t.Fatalf("new conflicts played %d sounds, want 1", len(got))
	}

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(300).Height(600).Background(theme.Background).Children(func() { app.projectGroup(c, p) })
	}, 300, 600)
	tt.Frame()
	label := "2 file(s) in conflict with its other copies: pick versions"
	if _, ok := tt.Find(label); !ok {
		t.Fatalf("no conflict mark in %q", tt.Texts())
	}
	tt.Click(label)
	tt.Frame()
	if app.linksForm == nil || app.linksForm.project.Path != p.Path {
		t.Fatal("the mark didn't open the project's sync dialog")
	}
}
