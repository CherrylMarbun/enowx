package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	dir := t.TempDir()
	info, err := cli.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: dir, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	slop, err := os.ReadFile("../core/unslop/testdata/slop.html")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	pageURL := "http://localhost:5173/dashboard"
	width, widths := 1200, []string{} // as the app shows it, and the widths asked
	look := "before"                  // what a screenshot shows
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
			u, text := pageURL, ""
			var image []byte
			switch {
			case req.Action == proto.BrowserScreenshot:
				image = []byte(look + "@" + strconv.Itoa(width))
			case req.Action == proto.BrowserWidth:
				text = strconv.Itoa(width)
				widths = append(widths, req.Text)
				if width, _ = strconv.Atoi(req.Text); width == 0 {
					width = 1200
				}
			case strings.Contains(req.Text, "clientWidth"):
				// At a phone's width it scrolls sideways.
				l := map[string]any{"width": width, "overflow": 0}
				if width == 390 {
					l["overflow"], l["off"] = 100, []string{"#hero"}
				}
				b, _ := json.Marshal(l)
				text = string(b)
			default:
				b, _ := json.Marshal(map[string]string{"url": u, "html": string(slop)})
				text = string(b)
			}
			mu.Unlock()
			_ = app.BrowserReply(proto.BrowserReply{ID: req.ID, Result: proto.BrowserResult{URL: u, Text: text, Image: image}})
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
	mu.Lock()
	widths = nil // the reads before measured every width too
	mu.Unlock()
	started, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Start: true})
	if err != nil || !strings.Contains(started.Ask, "browser unslop --pass N") || !strings.Contains(started.Ask, pageURL) {
		t.Fatalf("started: %v %s", err, started.Ask)
	}
	// Step by step: the steps told, then the first read.
	steps := 0
	r := next()
	for ; r.Scans == 0; r = next() {
		if len(r.Steps) != 5 {
			t.Fatalf("the steps: %+v", r.Steps)
		}
		steps++
	}
	if steps < 4 || r.ID != info.ID || r.From != started.Score || r.Theme != "dark" {
		t.Fatalf("the run after %d steps: %+v", steps, r)
	}
	mobile := false
	for _, f := range r.Findings {
		mobile = mobile || f.Rule == "overflow" && strings.Contains(f.Title, "phone (390 px)")
	}
	mu.Lock()
	asked := strings.Join(widths, ",")
	mu.Unlock()
	if !mobile || asked != "768,390,1200" {
		t.Fatalf("read at each width (asked %s), the phone's overflow found %v", asked, mobile)
	}
	// No pass before the brief; the brief is kept in the project.
	if _, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Pass: "1"}); err == nil || !strings.Contains(err.Error(), "## First screen") {
		t.Fatalf("a pass before the brief: %v", err)
	}
	if _, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Brief: "## Product\nAn API's docs"}); err == nil || !strings.Contains(err.Error(), "Users, Tone, First screen") {
		t.Fatalf("a brief lacking: %v", err)
	}
	brief := "## Product\nA read-only API's docs\n## Users\nDevelopers wiring a client\n## Tone\nCalm, dense\n## First screen\nThe endpoints"
	if _, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Brief: brief}); err != nil {
		t.Fatal(err)
	}
	if r := next(); r.Brief == nil || r.Brief.Tone != "Calm, dense" {
		t.Fatalf("the run's brief: %+v", r.Brief)
	}
	next() // the page read with it
	if kept, err := os.ReadFile(filepath.Join(dir, ".enowx", "brief.md")); err != nil || !strings.Contains(string(kept), "## Users\nDevelopers wiring a client") {
		t.Fatalf("kept: %s %v", kept, err)
	}
	cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Pass: "3"})
	if r := next(); r.Pass != "03 Spacing" || r.Scans != 3 || r.Brief == nil {
		t.Fatalf("pass 3: %+v", r)
	}
	if st, err := cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Status: true}); err != nil || st.Run.Pass != "03 Spacing" {
		t.Fatalf("status: %+v %v", st.Run, err)
	}

	// Pictured at each width when it starts, and again when done: the
	// before and after to compare.
	shots := func() string {
		t.Helper()
		got, err := cli.BrowserUnslopShots(info.ID)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, s := range got.Shots {
			out = append(out, s.Label+":"+string(s.Before)+">"+string(s.After))
		}
		return strings.Join(out, " ")
	}
	if got := shots(); got != "As shown:before@1200> Tablet:before@768> Phone:before@390>" {
		t.Fatalf("before: %s", got)
	}
	mu.Lock()
	look = "after"
	mu.Unlock()
	cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Pass: "done"})
	if r := next(); !r.Done || r.Shots != 3 {
		t.Fatalf("done: %+v", r)
	}
	if got := shots(); got != "As shown:before@1200>after@1200 Tablet:before@768>after@768 Phone:before@390>after@390" {
		t.Fatalf("before and after: %s", got)
	}

	// Done, a fresh agent reviews the screenshots; its verdict comes back
	// on the run, and below the bar a done again starts the next round.
	var reviewer string
	for r := next(); ; r = next() {
		if r.Review != nil && r.Review.Session != "" {
			reviewer = r.Review.Session
			break
		}
	}
	rs, err := cli.Sessions()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range rs {
		if s.ID != reviewer {
			continue
		}
		found = true
		if s.Name != "Review · round 1" || s.Group != "review:"+info.ID {
			t.Fatalf("the reviewer: %+v", s)
		}
		files, _ := os.ReadDir(s.Dir)
		if len(files) != 3 || !strings.HasSuffix(files[2].Name(), "-phone-390px.png") {
			t.Fatalf("what it reviews: %v", files)
		}
	}
	if !found {
		t.Fatalf("no reviewer session %s", reviewer)
	}
	if _, err := cli.DesignReview(proto.DesignReviewParams{Session: info.ID, Text: "Score: 6"}); err == nil {
		t.Fatal("a verdict from a session reviewing nothing was taken")
	}
	rv, err := cli.DesignReview(proto.DesignReviewParams{Session: reviewer, Text: "Score: 6\nIssue: hero: two CTAs → one\nKeep: the type"})
	if err != nil || rv.Round != 1 || rv.Passed || rv.Issues[0] != "hero: two CTAs → one" {
		t.Fatalf("the verdict: %+v %v", rv, err)
	}
	if r := next(); r.Review == nil || r.Review.Score != 6 || !r.Review.Done {
		t.Fatalf("the run's review: %+v", r.Review)
	}
	cli.BrowserUnslop(proto.BrowserUnslopParams{Session: info.ID, Pass: "done"})
	for r := next(); ; r = next() {
		if r.Review != nil && r.Review.Round == 2 {
			break
		}
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
	if ask := browserAsk("http://localhost:3000/", 10, 5, "dark", branch, base, false); !strings.Contains(ask, "going back to main undoes it") || !strings.Contains(ask, "Unslop: 03 Spacing") {
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
