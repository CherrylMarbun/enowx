package ui

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// Adding files from Files: the picker opens on this device, and what is
// picked lands in the folder of the session's folder, through its daemon.
func TestAddFilesThroughThePicker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	root := t.TempDir()
	s, err := cl.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: root, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	posted := make(chan func(), 64)
	app.onMain = func(fn func()) { posted <- fn }
	app.client = cl
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Background(theme.Background).Children(func() { app.localPick.View(c) })
	}, 800, 700)
	added := false
	app.addFiles(s, "docs", func() { added = true })
	tt.Frame()
	if err := tt.Click("logo.png"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	tt.Frame()
	if err := tt.Click("Add 1"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !added {
		select {
		case fn := <-posted:
			fn()
		case <-time.After(30 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("never added; status %q", app.status)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "docs", "logo.png")); err != nil || string(data) != "png" {
		t.Fatalf("in the session's folder: %q, %v; status %q", data, err, app.status)
	}
}
