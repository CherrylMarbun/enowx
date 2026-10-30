package daemon

import (
	"encoding/json"
	"os"
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
