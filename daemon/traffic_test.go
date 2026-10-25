package daemon

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
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

	"github.com/enowx/enowx/proto"
)

// clientFromEnv is an HTTP client set up as a program started with env
// would be: its proxy and its trust.
func clientFromEnv(t *testing.T, env []string) *http.Client {
	t.Helper()
	vars := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		vars[k] = v
	}
	pu, err := url.Parse(vars["HTTPS_PROXY"])
	if err != nil || pu.Host == "" {
		t.Fatalf("no proxy in %v", env)
	}
	pem, err := os.ReadFile(vars["NODE_EXTRA_CA_CERTS"])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(pem)
	return &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{RootCAs: roots}}}
}

// A session's traffic, caught through its new tabs' environment, shows
// through the daemon: listed by filter, read, mocked by the project's
// rules, paused and resumed, exported as HAR.
func TestTrafficThroughTheDaemon(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"path":%q}`, r.URL.Path)
	}))
	defer up.Close()
	srv := startServer(t)
	events := make(chan proto.TrafficSummary, 64)
	c := connect(t, srv, func(ev proto.Event) {
		var s proto.TrafficSummary
		if ev.Name == proto.EventTrafficExchange && json.Unmarshal(ev.Data, &s) == nil {
			events <- s
		}
	})
	project := t.TempDir()
	if _, err := c.AddProject(project); err != nil {
		t.Fatal(err)
	}
	sess, err := c.CreateSession(proto.SessionCreateParams{Project: project, Agent: "shell", BranchMode: proto.BranchNone, Name: "app"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TrafficAttach(proto.TrafficAttachParams{Kind: proto.TargetSession, Session: sess.ID}); err != nil {
		t.Fatal(err)
	}
	// A new tab gets the environment.
	tab, err := c.CreateSession(proto.SessionCreateParams{Parent: sess.ID, Agent: "custom", Command: []string{"sh", "-c", "echo $HTTPS_PROXY $NODE_EXTRA_CA_CERTS; sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	tabSess, _ := srv.sessions.Get(tab.ID)
	waitFor(t, "the tab's environment", func() bool { return strings.Contains(tabSess.Terminal().Text(), "http://127.0.0.1:") })
	env, err := c.TrafficEnv(proto.TrafficEnvParams{Session: sess.ID})
	if err != nil {
		t.Fatal(err)
	}
	client := clientFromEnv(t, env.Env)

	// The project's rules: a mock and a breakpoint.
	if _, err := c.SetTrafficRules(proto.TrafficRules{Project: project, Rules: []proto.TrafficRule{
		{Enabled: true, Match: proto.TrafficMatch{Path: "/mocked"}, Action: proto.TrafficAction{Kind: proto.ActionMock, Status: 503, Body: "down"}},
		{Enabled: true, Match: proto.TrafficMatch{Path: "/held"}, Action: proto.TrafficAction{Kind: proto.ActionBreakpoint, Break: "request"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(project, ".enowx", "traffic.json")); err != nil || !strings.Contains(string(b), "/mocked") {
		t.Fatalf("rules not saved in the project: %s %v", b, err)
	}
	get := func(path string) (int, string) {
		resp, err := client.Get(up.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if st, b := get("/api/users"); st != 200 || b != `{"path":"/api/users"}` {
		t.Fatalf("passed through: %d %s", st, b)
	}
	if st, b := get("/mocked"); st != 503 || b != "down" {
		t.Fatalf("mocked: %d %s", st, b)
	}
	done := make(chan string, 1)
	go func() { _, b := get("/held"); done <- b }()
	waitFor(t, "a paused request", func() bool { ps, _ := c.TrafficPaused(); return len(ps) == 1 })
	ps, _ := c.TrafficPaused()
	if err := c.TrafficResume(proto.TrafficResumeParams{ID: ps[0].ID, Edit: &proto.TrafficEdit{URL: up.URL + "/edited"}}); err != nil {
		t.Fatal(err)
	}
	if b := <-done; b != `{"path":"/edited"}` {
		t.Fatalf("resumed with an edit: %s", b)
	}

	waitFor(t, "three exchanges done", func() bool {
		l, _ := c.TrafficList(proto.TrafficListParams{Filter: "-is:pending"})
		return len(l.Exchanges) == 3
	})
	l, _ := c.TrafficList(proto.TrafficListParams{Filter: "path:/api/* status:2xx"})
	if len(l.Exchanges) != 1 || l.Exchanges[0].Session != sess.ID {
		t.Fatalf("filtered: %+v", l)
	}
	body, err := c.TrafficBody(proto.TrafficBodyParams{ID: l.Exchanges[0].ID, Which: "response"})
	if err != nil || string(body.Data) != `{"path":"/api/users"}` {
		t.Fatalf("body %q %v", body.Data, err)
	}
	if l, _ := c.TrafficList(proto.TrafficListParams{Filter: "is:mocked"}); len(l.Exchanges) != 1 || l.Exchanges[0].Status != 503 {
		t.Fatalf("mocked: %+v", l)
	}
	select {
	case <-events:
	case <-time.After(5 * time.Second):
		t.Fatal("no traffic events")
	}

	har, err := c.TrafficExport(proto.TrafficExportParams{Filter: "status:2xx"})
	if err != nil || !strings.Contains(string(har), `"/api/users\"}`) && !strings.Contains(string(har), "/api/users") {
		t.Fatalf("HAR: %s %v", har, err)
	}
	if err := c.TrafficClear(proto.TrafficClearParams{}); err != nil {
		t.Fatal(err)
	}
	if l, _ := c.TrafficList(proto.TrafficListParams{}); len(l.Exchanges) != 0 {
		t.Fatalf("after clear: %+v", l)
	}
	imported, err := c.TrafficImport(har)
	if err != nil || len(imported.Exchanges) != 2 {
		t.Fatalf("imported: %+v %v", imported, err)
	}

	// A request sent from the host is recorded too; loopback's
	// self-signed certificates aren't checked.
	sent, err := c.TrafficSend(proto.TrafficSendParams{Method: "GET", URL: up.URL + "/compose", Project: project, Direct: true})
	if err != nil || sent.Status != 200 || sent.Source != "compose" {
		t.Fatalf("sent: %+v %v", sent, err)
	}
}

// Containers started from the Docker tab get the proxy, through the
// bridge's gateway, and the CA.
func TestTrafficDockerWrapper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	os.WriteFile(filepath.Join(bin, "docker"), []byte("#!/bin/sh\necho \"real docker: $*\"\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	old := dockerGateway
	dockerGateway = func() (string, error) { return "127.0.0.1", nil }
	defer func() { dockerGateway = old }()
	srv := startServer(t)
	c := connect(t, srv, nil)
	sess, err := c.CreateSession(proto.SessionCreateParams{Agent: "shell", Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	tg, err := c.TrafficAttach(proto.TrafficAttachParams{Kind: proto.TargetDocker, Session: sess.ID})
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(srv.trafficDir(), "bin", tg.Source, "docker")
	out, err := exec.Command(wrapper, "run", "--rm", "alpine", "wget", "-qO-", "https://example.com").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	for _, want := range []string{"real docker: run --add-host=host.docker.internal:host-gateway", "HTTPS_PROXY=http://host.docker.internal:", "/etc/enowx-traffic/bundle.pem:ro", "alpine wget -qO- https://example.com"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in %s", want, out)
		}
	}
	if out, _ := exec.Command(wrapper, "ps").CombinedOutput(); strings.TrimSpace(string(out)) != "real docker: ps" {
		t.Errorf("other commands: %s", out)
	}
	if tg.Tab == "" {
		t.Error("no tab for docker")
	}
}

// enowX's MCP server goes into the agents set up on the host.
func TestTrafficMCPForAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	srv := startServer(t)
	c := connect(t, srv, nil)
	var res proto.TrafficMCP
	if err := c.Call(proto.MethodTrafficMCP, nil, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0] != "claude" {
		t.Fatalf("%+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	if !strings.Contains(string(b), `"enowx"`) || !strings.Contains(string(b), `"mcp"`) {
		t.Fatalf("Claude Code's config: %s", b)
	}
}
