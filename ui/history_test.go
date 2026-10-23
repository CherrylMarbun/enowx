package ui

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/transcript"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/ui/theme"
)

// Past conversations lists a project's, and Resume goes on with one in a
// new session.
func TestPastConversationsResume(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	folder := filepath.Join(home, ".claude", "projects", transcript.ClaudeFolder(repo))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	conv := `{"type":"user","uuid":"u1","cwd":"` + repo + `","message":{"content":"Add a dark theme"}}` + "\n" +
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"content":"Done."}}` + "\n"
	if err := os.WriteFile(filepath.Join(folder, "abc123.jsonl"), []byte(conv), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho \"args: $*\"\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	proj, err := cl.AddProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	posted := make(chan func(), 16)
	app.onMain = func(fn func()) { posted <- fn }
	app.client = cl
	app.openHistory(proj)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(700).Height(600).Background(theme.Background).Children(func() { app.historyDialogView(c) })
	}, 700, 600)
	drain := func(until func() bool, what string) {
		deadline := time.Now().Add(10 * time.Second)
		for !until() {
			select {
			case fn := <-posted:
				fn()
			case <-time.After(50 * time.Millisecond):
			}
			tt.Frame()
			if time.Now().After(deadline) {
				t.Fatal("never: " + what)
			}
		}
	}
	drain(func() bool { return !app.history.loading }, "the list")
	if !tt.HasText("Add a dark theme") || !tt.HasText("Resume") {
		t.Fatalf("dialog: %q", tt.Texts())
	}
	shot(t, tt, "history.png")
	if err := tt.Click("Resume"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return app.selected != "" }, "the resumed session")
	waitScreen := time.Now().Add(10 * time.Second)
	for {
		text, _ := cl.Screen(app.selected)
		if strings.Contains(text, "--resume abc123") {
			break
		}
		if time.Now().After(waitScreen) {
			t.Fatalf("screen: %q", text)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
