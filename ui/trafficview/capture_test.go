package trafficview

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// Pause from the toolbar: the app's requests still go through, the list
// says how many weren't recorded and, once resumed, where the gap was;
// Stop and Start from the keyboard.
func TestTrafficPausesFromTheTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer up.Close()
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	queue := make(chan func(), 256)
	a, b := net.Pipe()
	go srv.ServeConn(a)
	var v *View
	cl, err := client.New(b, func(ev proto.Event) {
		switch ev.Name {
		case proto.EventTrafficExchange:
			var s proto.TrafficSummary
			if json.Unmarshal(ev.Data, &s) == nil {
				queue <- func() { v.Apply(s) }
			}
		case proto.EventTrafficCapture:
			var st proto.TrafficCapture
			if json.Unmarshal(ev.Data, &st) == nil {
				queue <- func() { v.SetCapture(st) }
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	project := t.TempDir()
	cl.AddProject(project)
	sess, err := cl.CreateSession(proto.SessionCreateParams{Project: project, Agent: "shell", BranchMode: proto.BranchNone, Name: "app"})
	if err != nil {
		t.Fatal(err)
	}
	env, err := cl.TrafficEnv(proto.TrafficEnvParams{Session: sess.ID})
	if err != nil {
		t.Fatal(err)
	}
	var proxy string
	for _, kv := range env.Env {
		if p, ok := strings.CutPrefix(kv, "HTTPS_PROXY="); ok {
			proxy = p
		}
	}
	pu, _ := url.Parse(proxy)
	hc := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	get := func(path string) {
		t.Helper()
		resp, err := hc.Get(up.URL + path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		resp.Body.Close()
	}

	v = New()
	o := Options{Host: cl, HostName: "This PC", Session: sess, Update: func(fn func()) { queue <- fn }, CopyText: func(string) {}}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(560).Height(520).Background(theme.Background).TextColor(theme.Text).Children(func() { v.Draw(c, o) })
	}, 560, 520)
	drain := func(until func() bool, what string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !until() {
			select {
			case fn := <-queue:
				fn()
			case <-time.After(30 * time.Millisecond):
			}
			tt.Frame()
			if time.Now().After(deadline) {
				t.Fatalf("never: %s (%q)", what, tt.Texts())
			}
		}
	}
	drain(func() bool { return v.loaded && v.capture.loaded && tt.HasText("Recording") }, "loaded, recording")
	get("/before")
	drain(func() bool { return tt.HasText(strings.TrimPrefix(up.URL, "https://") + "/before") }, "the first request")

	if err := tt.Click("Pause recording (⌘⌥P)"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return tt.HasText("Pause everything") }, "the pause menu")
	if err := tt.Click("Pause everything"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return v.capture.st.State == proto.CapturePaused }, "paused")
	get("/while-paused")
	drain(func() bool { return tt.HasText("Paused · 1 not recorded") }, "the count not recorded")
	shot(t, tt, "traffic-paused.png")

	if err := tt.Click("Resume recording (⌘⌥P)"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return v.capture.st.State == proto.CaptureRecording }, "recording again")
	get("/after")
	drain(func() bool {
		texts := strings.Join(tt.Texts(), "\n")
		return strings.Contains(texts, "/after") && strings.Contains(texts, "1 not recorded —") && !strings.Contains(texts, "/while-paused")
	}, "the gap in the list")
	shot(t, tt, "traffic-gap.png")

	tt.Key(ui.Cmd|ui.Alt, ui.KeyS)
	drain(func() bool { return tt.HasText("Stopped") && !v.capture.busy }, "stopped from the keyboard")
	get("/while-stopped")
	tt.Key(ui.Cmd|ui.Alt, ui.KeyS)
	drain(func() bool { return v.capture.st.State == proto.CaptureRecording }, "started from the keyboard")
}
