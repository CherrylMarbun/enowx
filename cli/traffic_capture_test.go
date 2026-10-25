package cli_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Pausing, stopping and starting from the command line: traffic keeps
// flowing, unrecorded while paused or stopped, status says which, and
// the defaults are kept.
func TestTrafficPauseCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "enowxd")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/enowx/enowx/cmd/enowxd").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home, err := os.MkdirTemp("", "enx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(home)
	env := append(os.Environ(), "ENOWX_HOME="+home, "ENOWX_SESSION=")
	serve := exec.Command(bin, "serve")
	serve.Env = env
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stop := exec.Command(bin, "stop")
		stop.Env = env
		_ = stop.Run()
		_ = serve.Wait()
	}()
	run := func(args ...string) string {
		t.Helper()
		var out []byte
		var err error
		for range 50 {
			cmd := exec.Command(bin, args...)
			cmd.Env = env
			if out, err = cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "isn't running") {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			t.Fatalf("enowxd %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "ok") }))
	defer up.Close()
	proxy := ""
	for _, line := range strings.Split(run("traffic", "env"), "\n") {
		if v, ok := strings.CutPrefix(line, "export HTTPS_PROXY="); ok {
			proxy = strings.Trim(v, "'")
		}
	}
	pu, _ := url.Parse(proxy)
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	get := func(path string) {
		t.Helper()
		resp, err := client.Get(up.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
	}

	if out := run("traffic", "pause", "--rules", "pause"); !strings.Contains(out, "paused since") || !strings.Contains(out, "rules paused") {
		t.Fatalf("pause: %s", out)
	}
	get("/while-paused")
	if out := run("traffic", "status"); !strings.Contains(out, "1 not recorded") {
		t.Fatalf("status while paused: %s", out)
	}
	if out := run("traffic", "watch", "--all", "--timeout", "1s"); !strings.Contains(out, "recording is paused") {
		t.Errorf("watch while paused: %s", out)
	}
	run("traffic", "stop")
	get("/while-stopped")
	if out := run("traffic", "start"); !strings.Contains(out, "recording") || !strings.Contains(out, "stopped") {
		t.Fatalf("start: %s", out)
	}
	get("/recorded")
	time.Sleep(300 * time.Millisecond)
	list := run("traffic", "list", "--all")
	if !strings.Contains(list, "/recorded") || strings.Contains(list, "/while-") {
		t.Fatalf("list: %s", list)
	}
	run("traffic", "resume") // records already: nothing to do
	if out := run("traffic", "prefs", "--auto-count", "100"); !strings.Contains(out, "after 100 exchanges") {
		t.Fatalf("prefs: %s", out)
	}
	if b, err := os.ReadFile(filepath.Join(home, "settings", "traffic.json")); err != nil || !strings.Contains(string(b), `"autoCount": 100`) {
		t.Fatalf("synced prefs: %s, %v", b, err)
	}
}
