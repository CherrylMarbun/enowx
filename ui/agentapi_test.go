//go:build !windows

package ui

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/ui/theme"
)

// Settings → Model APIs points Claude Code at an API in a dialog, then
// shows it set, its key masked.
func TestPointAnAgentAtAModelAPI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin") // only this claude is installed
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
	app := New(Options{})
	posted := make(chan func(), 64)
	app.onMain = func(fn func()) { posted <- fn }
	app.client = cl
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(760).Height(900).Background(theme.Background).Children(func() { app.agentAPISection(c) })
	}, 760, 900)
	frames := func(until func() bool, what string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !until() {
			select {
			case fn := <-posted:
				fn()
			case <-time.After(30 * time.Millisecond):
			}
			tt.Frame()
			if time.Now().After(deadline) {
				t.Fatalf("never: %s; %q", what, tt.Texts())
			}
		}
	}
	tt.Frame()
	frames(func() bool { return app.agentAPIs.loaded && tt.HasText("Claude Code") }, "the agents")
	shot(t, tt, "model-apis.png")
	if err := tt.Click("Use an API…"); err != nil {
		t.Fatalf("%v: %q", err, tt.Texts())
	}
	frames(func() bool { return tt.HasText("Base URL") }, "the dialog")
	f := app.agentAPIs.edit
	if f.agent.Agent != "claude" {
		t.Fatalf("the dialog is %s's", f.agent.Agent)
	}
	f.url, f.key = "https://gw.example.com", "sk-test-9876"
	f.models[0].id, f.models[0].output = "gw-sonnet", "32000"
	if err := tt.Click("Add a model"); err != nil {
		t.Fatal(err)
	}
	f.models[1].id, f.models[1].effort = "gw-opus", 2 // low, medium, high
	tt.Frame()
	shot(t, tt, "model-api-dialog.png")
	if err := tt.Click("Use it"); err != nil {
		t.Fatal(err)
	}
	frames(func() bool { return app.agentAPIs.edit == nil && tt.HasText("https://gw.example.com · gw-sonnet +1 · key ••••9876") }, "Claude Code set")
	data, _ := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	for _, want := range []string{`"ANTHROPIC_BASE_URL": "https://gw.example.com"`, `"CLAUDE_CODE_MAX_OUTPUT_TOKENS": "32000"`, `"model": "gw-opus"`, `"effortLevel": "high"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("no %s in settings:\n%s", want, data)
		}
	}
	if !strings.Contains(string(data), `"ANTHROPIC_MODEL": "gw-sonnet"`) {
		t.Fatalf("settings:\n%s", data)
	}
	shot(t, tt, "model-apis-set.png")
}
