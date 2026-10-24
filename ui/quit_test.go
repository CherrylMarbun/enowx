//go:build !windows

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
)

// Quitting with the agents to stop stops this device's agents through its
// daemon's socket, keeping their sessions.
func TestQuitStopsThisDevicesAgents(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho ready\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ENOWX_HOME", t.TempDir())
	path, err := transport.SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(path)) })
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Serve(l)

	cl, err := localClient()
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	info, err := cl.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{})
	a.stopOnQuit = true
	a.stopLocalAgents()
	list, err := cl.Sessions()
	if err != nil || len(list) != 1 || list[0].ID != info.ID || !list[0].Hibernated {
		t.Fatalf("after quitting: %+v, %v", list, err)
	}
}
