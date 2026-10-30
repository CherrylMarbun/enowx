package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// Unslop in the Browser: the page the app shows (its HTML and styles, as
// the page's script gives them) is read on localhost only; started, a run
// begins with what to tell the session's agent, and its passes come back
// as events. A page elsewhere is refused.
func TestBrowserUnslop(t *testing.T) {
	srv := startServer(t)
	cli := connect(t, srv, nil)
	info, err := cli.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	slop, err := os.ReadFile("../core/unslop/testdata/slop.html")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	pageURL := "http://localhost:5173/dashboard"
	runs := make(chan proto.CanvasUnslopRun, 8)
	var app *client.Client
	app = connect(t, srv, func(ev proto.Event) {
		var run proto.CanvasUnslopRun
		if ev.Name == proto.EventBrowserUnslop && json.Unmarshal(ev.Data, &run) == nil {
			runs <- run
		}
		if ev.Name != proto.EventBrowserRequest {
			return
		}
		var req proto.BrowserRequest
		if json.Unmarshal(ev.Data, &req) != nil {
			return
		}
		go func() {
			mu.Lock()
			u := pageURL
			mu.Unlock()
			page, _ := json.Marshal(map[string]string{"url": u, "html": string(slop)})
			_ = app.BrowserReply(proto.BrowserReply{ID: req.ID, Result: proto.BrowserResult{URL: u, Text: string(page)}})
		}()
	})
	if err := app.ServeBrowsers(); err != nil {
		t.Fatal(err)
	}
	next := func() proto.CanvasUnslopRun {
		t.Helper()
		select {
		case r := <-runs:
			return r
		case <-time.After(5 * time.Second):
			t.Fatal("no run event")
		}
		return proto.CanvasUnslopRun{}
	}

	// Asked by the app that serves the Browser, on its own connection, as
	// its Unslop button does: the page is read all the same.
	if _, err := app.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID}); err != nil {
		t.Fatalf("from the app serving the Browser: %v", err)
	}
	read, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID})
	if err != nil || read.Frame != pageURL || read.Score > 20 || len(read.Findings) < 10 || read.Ask != "" {
		t.Fatalf("read: %+v %v", read.Frame, err)
	}
	started, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Start: true})
	if err != nil || !strings.Contains(started.Ask, "browser unslop --pass N") || !strings.Contains(started.Ask, pageURL) {
		t.Fatalf("started: %v %s", err, started.Ask)
	}
	if r := next(); r.ID != info.ID || r.From != started.Score || r.Theme != "dark" {
		t.Fatalf("the run: %+v", r)
	}
	cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Pass: "3"})
	if r := next(); r.Pass != "03 Spacing" || r.Scans != 2 {
		t.Fatalf("pass 3: %+v", r)
	}
	if st, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Status: true}); err != nil || st.Run.Pass != "03 Spacing" {
		t.Fatalf("status: %+v %v", st.Run, err)
	}

	mu.Lock()
	pageURL = "https://example.com/"
	mu.Unlock()
	if _, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID}); err == nil || !strings.Contains(err.Error(), "localhost") {
		t.Fatalf("a page elsewhere: %v", err)
	}
}

// Only pages on this device are read.
func TestLocalPage(t *testing.T) {
	for u, want := range map[string]bool{
		"http://localhost:5173/": true, "http://127.0.0.1:3000": true, "http://[::1]:8080/x": true, "http://app.localhost/": true,
		"https://example.com/": false, "http://192.168.1.4:3000": false, "file:///etc/passwd": false, "http://localhost.evil.com/": false,
	} {
		if got := localPage(u); got != want {
			t.Errorf("%s: %v, want %v", u, got, want)
		}
	}
}

// A Browser unslop in a git project starts on a branch of its own, made
// from the one checked out; with changes not committed it is refused, so
// that branch holds the redesign alone. Outside git it goes on without.
func TestBrowserUnslopBranch(t *testing.T) {
	srv := startServer(t)
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "page.html"), []byte("<p>hi</p>"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-qm", "first").Run()
	cli := connect(t, srv, nil)
	info, err := cli.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: dir, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(dir, "page.html"), []byte("<p>changed</p>"), 0o644)
	if _, _, err := srv.unslopBranch(info.ID); err == nil || !strings.Contains(err.Error(), "not committed") {
		t.Fatalf("with changes not committed: %v", err)
	}
	exec.Command("git", "-C", dir, "checkout", "--", ".").Run()
	branch, base, err := srv.unslopBranch(info.ID)
	if err != nil || base != "main" || !strings.HasPrefix(branch, "unslop/") {
		t.Fatalf("branch %q from %q: %v", branch, base, err)
	}
	if out, _ := exec.Command("git", "-C", dir, "branch", "--show-current").Output(); strings.TrimSpace(string(out)) != branch {
		t.Fatalf("checked out: %s", out)
	}
	if ask := browserAsk("http://localhost:3000/", 10, 5, "dark", branch, base); !strings.Contains(ask, "going back to main undoes it") || !strings.Contains(ask, "Unslop: 03 Spacing") {
		t.Fatalf("the ask: %s", ask)
	}

	plain, err := cli.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	if b, _, err := srv.unslopBranch(plain.ID); err != nil || b != "" {
		t.Fatalf("outside git: %q %v", b, err)
	}
}
