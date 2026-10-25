package cli_test

import (
	"crypto/tls"
	"fmt"
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

// The traffic commands: a shell's environment, the list, a mock added
// from the command line, one exchange shown, a HAR exported.
func TestTrafficCommands(t *testing.T) {
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
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"path":%q}`, r.URL.Path)
	}))
	defer up.Close()

	exports := run("traffic", "env")
	proxy := ""
	for _, line := range strings.Split(exports, "\n") {
		if v, ok := strings.CutPrefix(line, "export HTTPS_PROXY="); ok {
			proxy = strings.Trim(v, "'")
		}
	}
	pu, err := url.Parse(proxy)
	if err != nil || pu.Host == "" {
		t.Fatalf("exports: %s", exports)
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	get := func(path string) (int, string) {
		resp, err := client.Get(up.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if st, _ := get("/api/items"); st != 200 {
		t.Fatalf("status %d", st)
	}
	project := t.TempDir()
	run("traffic", "mock", "--project", project, "--path", "/api/fail", "--status", "418", "--body", "teapot")
	// Rules apply to sources of the project: this one's is the host's,
	// so the mock goes there too.
	run("traffic", "mock", "--path", "/api/fail", "--status", "418", "--body", "teapot")
	if st, b := get("/api/fail"); st != 418 || b != "teapot" {
		t.Fatalf("mock: %d %s", st, b)
	}
	var list string
	for deadline := time.Now().Add(5 * time.Second); ; {
		list = run("traffic", "list", "--all", "--filter", "path:/api/*")
		if strings.Count(list, "\n") >= 1 && !strings.Contains(list, "...") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("list: %s", list)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(list, "/api/items") || !strings.Contains(list, "418") || !strings.Contains(list, "mocked") {
		t.Fatalf("list: %s", list)
	}
	show := run("traffic", "show", "1")
	if !strings.Contains(show, `{"path":"/api/items"}`) {
		t.Fatalf("show: %s", show)
	}
	if rules := run("traffic", "rules"); !strings.Contains(rules, "/api/fail → mock 418") {
		t.Fatalf("rules: %s", rules)
	}
	har := filepath.Join(tmp, "out.har")
	run("traffic", "export", "--all", "--out", har)
	if b, err := os.ReadFile(har); err != nil || !strings.Contains(string(b), "/api/items") {
		t.Fatalf("HAR: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(project, ".enowx", "traffic.json")); !strings.Contains(string(b), "/api/fail") {
		t.Fatalf("project rules: %s", b)
	}

	// The same, as an agent's MCP tools.
	mcp := exec.Command(bin, "mcp")
	mcp.Env = env
	mcp.Stdin = strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"traffic_list","arguments":{"all":true,"filter":"path:/api/*"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"traffic_mock","arguments":{"path":"/api/new","status":201,"body":"made","project":""}}}`,
	}, "\n") + "\n")
	out, err := mcp.Output()
	if err != nil {
		t.Fatalf("mcp: %v\n%s", err, out)
	}
	replies := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(replies) != 4 {
		t.Fatalf("replies: %s", out)
	}
	for _, want := range []string{`"serverInfo":{"name":"enowx"`, `"name":"traffic_watch"`, `/api/items`, `"isError":false`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	if st, b := get("/api/new"); st != 201 || b != "made" {
		t.Fatalf("mock from MCP: %d %s", st, b)
	}
}
