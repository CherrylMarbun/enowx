package ui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/hostconn"
)

// Choosing root sticks: in the host's form, and on connecting again with
// a copy of the host from before the choice (a retry's), which used to
// ask again and pick the regular user enowX had set up.
func TestRunningAsRootSticks(t *testing.T) {
	store, err := hosts.Open(filepath.Join(t.TempDir(), "hosts.json"))
	if err != nil {
		t.Fatal(err)
	}
	stale, err := store.Add(hosts.Host{Name: "vps", Address: "root@127.0.0.1:1", RunAs: "enowx"})
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{Hosts: store})
	a.onMain = func(func()) {}
	a.openEditHost(stale)
	if a.addHost.asRoot || a.addHost.runAs != "enowx" {
		t.Fatalf("form %+v", a.addHost)
	}
	a.addHost.asRoot = true
	a.saveHost()
	saved, _ := store.Get(stale.ID)
	if !saved.KeepRoot || saved.RunAs != "" {
		t.Fatalf("saved %+v", saved)
	}
	a.connect(stale)
	if !a.host.KeepRoot || a.host.RunAs != "" {
		t.Fatalf("connected with %+v, not as saved", a.host)
	}
}

// Asking which user to run as asks, even when a user enowX ran as before
// is there, which it otherwise picks without asking.
func TestAskingWhomToRunAsAsks(t *testing.T) {
	store, err := hosts.Open(filepath.Join(t.TempDir(), "hosts.json"))
	if err != nil {
		t.Fatal(err)
	}
	h, _ := store.Add(hosts.Host{Name: "vps", Address: "root@203.0.113.1"})
	a := New(Options{Hosts: store})
	main := make(chan func(), 4)
	a.onMain = func(fn func()) { main <- fn }
	users := []hostconn.ServerUser{{Name: "enowx", Home: "/home/enowx", HasDaemon: true}}

	a.chooseRunAs = h.ID
	done := make(chan hosts.Host, 1)
	go func() {
		got, _ := a.askRunAs(a.gen, h, users)
		done <- got
	}()
	(<-main)()
	if a.runAs == nil || a.runAs.choice != "enowx" {
		t.Fatalf("not asked: %+v", a.runAs)
	}
	a.answerRunAs("", true) // Keep root
	if got := <-done; !got.KeepRoot {
		t.Fatalf("answered %+v", got)
	}
	if saved, _ := store.Get(h.ID); !saved.KeepRoot {
		t.Fatalf("saved %+v", saved)
	}

	// Without asking, the user set up before is picked, as before.
	go func() {
		got, _ := a.askRunAs(a.gen, h, users)
		done <- got
	}()
	(<-main)()
	select {
	case got := <-done:
		if got.RunAs != "enowx" {
			t.Fatalf("picked %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("asked without being asked to")
	}
}
