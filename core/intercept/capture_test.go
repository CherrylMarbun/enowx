package intercept

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// Paused, traffic still flows and rules run (or not, as asked) but nothing
// is recorded; stopped, it flows as if there were no proxy; resumed, it's
// recorded again, the pause kept as a gap.
func TestCapturePausesAndStops(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "real "+r.URL.Path) }))
	defer up.Close()
	p, _, client := testProxy(t, []proto.TrafficRule{
		{ID: "m", Enabled: true, Match: proto.TrafficMatch{Path: "/mock"}, Action: proto.TrafficAction{Kind: proto.ActionMock, Status: 200, Body: "mocked"}},
	})
	var events []proto.TrafficCapture
	p.OnCapture = func(st proto.TrafficCapture) { events = append(events, st) }
	get := func(path string) string {
		t.Helper()
		resp, err := client.Get(up.URL + path)
		if err != nil {
			t.Fatalf("%s through a %s proxy: %v", path, p.Capture().State, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	recorded := func() int { return len(p.Store.All()) }

	get("/a")
	waitDone(t, p, 1)
	if _, err := p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause}); err != nil {
		t.Fatal(err)
	}
	if got := get("/mock"); got != "mocked" {
		t.Errorf("paused with its rules kept, /mock = %q", got)
	}
	if got := get("/b"); got != "real /b" || recorded() != 1 {
		t.Errorf("paused: %q, %d recorded", got, recorded())
	}
	p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause, Rules: "pause"})
	if got := get("/mock"); got != "real /mock" {
		t.Errorf("paused with its rules, /mock = %q", got)
	}
	p.SetCapture(proto.TrafficCaptureParams{Action: proto.CaptureStop})
	if got := get("/mock"); got != "real /mock" || recorded() != 1 {
		t.Errorf("stopped: %q, %d recorded", got, recorded())
	}
	st, _ := p.SetCapture(proto.TrafficCaptureParams{Action: proto.CaptureStart})
	get("/c")
	waitDone(t, p, 2)
	if st.State != proto.CaptureRecording || len(st.Gaps) != 3 || st.Gaps[0].Skipped != 2 || st.Gaps[2].State != proto.CaptureStopped {
		t.Errorf("after starting again: %+v", st)
	}
	if len(events) < 4 {
		t.Errorf("%d capture events", len(events))
	}
}

// A pause can take in some traffic only: a filter's, or some sources'.
func TestCapturePausesPartly(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer up.Close()
	p, src, client := testProxy(t, nil)
	get := func(path string) {
		t.Helper()
		resp, err := client.Get(up.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause, Filter: "path:/analytics"})
	get("/analytics/hit")
	get("/api/items")
	got := waitDone(t, p, 1)
	if len(got) != 1 || !strings.HasSuffix(got[0].URL, "/api/items") {
		t.Errorf("recorded while analytics was paused: %+v", got)
	}
	p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause, Sources: []string{"browser"}})
	get("/api/more") // a session's: still recorded
	waitDone(t, p, 2)
	p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause, Sources: []string{src.Session}})
	get("/api/again")
	time.Sleep(100 * time.Millisecond)
	if n := len(p.Store.All()); n != 2 {
		t.Errorf("%d recorded with the session paused", n)
	}
	if _, err := p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause, Filter: "status:abc"}); err == nil {
		t.Error("a bad filter paused")
	}
}

// An exchange held at a breakpoint goes on when recording pauses, and
// none is held while paused, where it couldn't be seen.
func TestCapturePauseReleasesBreakpoints(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer up.Close()
	p, _, client := testProxy(t, []proto.TrafficRule{
		{ID: "b", Enabled: true, Match: proto.TrafficMatch{Path: "/held"}, Action: proto.TrafficAction{Kind: proto.ActionBreakpoint}},
	})
	done := make(chan string, 1)
	go func() {
		resp, err := client.Get(up.URL + "/held")
		if err != nil {
			done <- err.Error()
			return
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		done <- string(b)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for len(p.Paused()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("never held")
		}
		time.Sleep(10 * time.Millisecond)
	}
	p.SetCapture(proto.TrafficCaptureParams{Action: proto.CapturePause})
	select {
	case got := <-done:
		if got != "ok" {
			t.Errorf("released: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still held after pausing")
	}
	resp, err := client.Get(up.URL + "/held")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

// Recording pauses on its own after the exchanges the preferences allow.
func TestCapturePausesOnItsOwn(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer up.Close()
	p, _, client := testProxy(t, nil)
	p.SetCapturePrefs(proto.TrafficCapturePrefs{AutoCount: 2})
	for i := 0; i < 4; i++ {
		resp, err := client.Get(up.URL + "/x")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
	st := p.Capture()
	if st.State != proto.CapturePaused || !strings.Contains(st.Reason, "2 exchanges") || st.Skipped != 2 || len(p.Store.All()) != 2 {
		t.Errorf("after 4 with 2 allowed: %+v, %d recorded", st, len(p.Store.All()))
	}
}
