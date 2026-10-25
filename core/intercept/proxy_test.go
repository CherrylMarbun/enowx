package intercept

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// testProxy is a proxy on loopback with one source, and a client sending
// through it that trusts the proxy's CA.
func testProxy(t *testing.T, rules []proto.TrafficRule) (*Proxy, Source, *http.Client) {
	t.Helper()
	ca, err := NewCA(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	compiled := CompileRules(rules, nil)
	p := &Proxy{CA: ca, Store: NewStore(), RulesFor: func(Source) *Rules { return compiled }, Insecure: func(string) bool { return true }}
	if err := p.Listen("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	src := p.AddSource("session", "test", "s1", "")
	pu, _ := url.Parse(p.URL(src))
	roots := x509.NewCertPool()
	roots.AddCert(ca.Cert)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{RootCAs: roots}, ForceAttemptHTTP2: true}, Timeout: 10 * time.Second}
	return p, src, client
}

func waitDone(t *testing.T, p *Proxy, n int) []proto.TrafficSummary {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var out []proto.TrafficSummary
		done := 0
		for _, ex := range p.Store.All() {
			s := ex.Summary()
			out = append(out, s)
			if s.State == proto.TrafficDone || s.State == proto.TrafficFailed {
				done++
			}
		}
		if done >= n {
			return out
		}
		if time.Now().After(deadline) {
			t.Fatalf("exchanges: %+v", out)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// HTTPS through the proxy, over HTTP/1.1 and HTTP/2, is recorded with its
// bodies decoded, attributed to its source, its secrets masked.
func TestProxyRecordsHTTPS(t *testing.T) {
	up := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"path":%q,"got":%q,"proto":%q}`, r.URL.Path, body, r.Proto)
	}))
	up.EnableHTTP2 = true
	up.StartTLS()
	defer up.Close()
	p, src, client := testProxy(t, nil)
	req, _ := http.NewRequest("POST", up.URL+"/api/items?token=abcdefghijklmnopqrst", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Authorization", "Bearer abcdefghijklmnopqrstuvwxyz")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(got), `"got":"{\"name\":\"x\"}"`) || resp.ProtoMajor != 2 {
		t.Fatalf("response %s %s", resp.Proto, got)
	}
	list := waitDone(t, p, 1)
	s := list[0]
	if s.Source != src.ID || s.Session != "s1" || s.Method != "POST" || s.Status != 200 || s.Type != "json" || !strings.HasPrefix(s.Proto, "HTTP/2") {
		t.Fatalf("summary %+v", s)
	}
	ex, _ := p.Store.Get(s.ID)
	d := ex.Details(false)
	for _, h := range d.ReqHeaders {
		if h.Name == "Authorization" && strings.Contains(h.Value, "abcdefghij") {
			t.Fatalf("authorization not masked: %q", h.Value)
		}
	}
	if strings.Contains(d.URL, "abcdefghij") {
		t.Fatalf("token in URL not masked: %s", d.URL)
	}
	if d.TLS == nil || d.TLS.ALPN != "h2" {
		t.Fatalf("tls %+v", d.TLS)
	}
	if b := ex.rawBody("request"); string(b.data) != `{"name":"x"}` {
		t.Fatalf("request body %q", b.data)
	}
}

// Without its credentials, the proxy refuses.
func TestProxyNeedsCredentials(t *testing.T) {
	p, _, _ := testProxy(t, nil)
	pu, _ := url.Parse("http://nobody:wrong@" + p.Addr())
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
	resp, err := client.Get("http://example.invalid/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusProxyAuthRequired {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// Rules mock, transform, fail and pause exchanges.
func TestProxyRules(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"user":{"name":"real","role":"user"},"agent":%q}`, r.Header.Get("X-Added"))
	}))
	defer up.Close()
	upHost := strings.TrimPrefix(up.URL, "http://")
	newName := `{"user":{"name":"edited"}}`
	p, _, client := testProxy(t, []proto.TrafficRule{
		{ID: "m", Enabled: true, Match: proto.TrafficMatch{Path: "/mock"}, Action: proto.TrafficAction{Kind: proto.ActionMock, Status: 418, Body: `{"mocked":true}`, Times: 1}},
		{ID: "t", Enabled: true, Match: proto.TrafficMatch{Path: "/transform", Methods: []string{"GET"}}, Action: proto.TrafficAction{Kind: proto.ActionTransform,
			Request:  &proto.TrafficTransform{SetHeaders: []proto.Header{{Name: "X-Added", Value: "yes"}}},
			Response: &proto.TrafficTransform{JSON: map[string]string{"user.role": `"admin"`}, Status: 201}}},
		{ID: "f", Enabled: true, Match: proto.TrafficMatch{Path: "/fail"}, Action: proto.TrafficAction{Kind: proto.ActionFail, Fail: "status", Status: 502}},
		{ID: "b", Enabled: true, Match: proto.TrafficMatch{Path: "/pause"}, Action: proto.TrafficAction{Kind: proto.ActionBreakpoint, Break: "response"}},
		{ID: "body", Enabled: true, Match: proto.TrafficMatch{JSON: map[string]string{"kind": "special"}}, Action: proto.TrafficAction{Kind: proto.ActionMock, Body: "special"}},
	})
	get := func(path string) (int, string) {
		resp, err := client.Get(up.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if st, b := get("/mock"); st != 418 || b != `{"mocked":true}` {
		t.Fatalf("mock: %d %s", st, b)
	}
	if st, _ := get("/mock"); st != 200 {
		t.Fatalf("a mock used once answered again: %d", st)
	}
	if st, b := get("/transform"); st != 201 || !strings.Contains(b, `"role":"admin"`) || !strings.Contains(b, `"agent":"yes"`) {
		t.Fatalf("transform: %d %s", st, b)
	}
	if st, _ := get("/fail"); st != 502 {
		t.Fatalf("fail: %d", st)
	}
	resp, err := client.Post("http://"+upHost+"/any", "application/json", strings.NewReader(`{"kind":"special"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "special" {
		t.Fatalf("body match: %s", b)
	}
	done := make(chan string)
	go func() { _, b := get("/pause"); done <- b }()
	var paused []proto.TrafficPaused
	for deadline := time.Now().Add(5 * time.Second); len(paused) == 0; {
		if time.Now().After(deadline) {
			t.Fatal("never paused")
		}
		time.Sleep(20 * time.Millisecond)
		paused = p.Paused()
	}
	body := []byte(newName)
	if err := p.Resume(paused[0].ID, false, &proto.TrafficEdit{Body: &body}); err != nil {
		t.Fatal(err)
	}
	if b := <-done; b != newName {
		t.Fatalf("edited at the breakpoint: %s", b)
	}
}

// A WebSocket is relayed, its messages recorded both ways.
func TestProxyWebSocket(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sum := sha1Accept(r.Header.Get("Sec-WebSocket-Key"))
		c, brw, _ := w.(http.Hijacker).Hijack()
		defer c.Close()
		writeHead(c, 101, http.Header{"Upgrade": {"websocket"}, "Connection": {"Upgrade"}, "Sec-Websocket-Accept": {sum}})
		f, err := readFrame(brw.Reader)
		if err != nil {
			return
		}
		writeFrame(c, wsText, append([]byte("echo: "), f.payload...))
		readFrame(brw.Reader)
	}))
	defer up.Close()
	p, src, _ := testProxy(t, nil)
	c, err := net.Dial("tcp", p.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	host := strings.TrimPrefix(up.URL, "http://")
	auth := base64.StdEncoding.EncodeToString([]byte(src.ID + ":" + src.Token))
	fmt.Fprintf(c, "GET http://%s/ws HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", host, host, auth)
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != 101 {
		t.Fatalf("upgrade: %v %v", resp, err)
	}
	// A client's frame is masked.
	msg := []byte("hello")
	key := []byte{1, 2, 3, 4}
	masked := make([]byte, len(msg))
	for i := range msg {
		masked[i] = msg[i] ^ key[i%4]
	}
	c.Write(append([]byte{0x81, 0x80 | byte(len(msg))}, append(key, masked...)...))
	f, err := readFrame(br)
	if err != nil || string(f.payload) != "echo: hello" {
		t.Fatalf("frame %q %v", f.payload, err)
	}
	c.Write([]byte{0x88, 0x80, 0, 0, 0, 0})
	c.Close()
	list := waitDone(t, p, 1)
	ex, _ := p.Store.Get(list[0].ID)
	msgs := ex.Messages(0, 0)
	if len(msgs) < 2 || !msgs[0].Sent || string(msgs[0].Data) != "hello" || msgs[1].Sent || string(msgs[1].Data) != "echo: hello" {
		t.Fatalf("messages %+v", msgs)
	}
}

func sha1Accept(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// SOCKS5 with the source's credentials reaches HTTPS like CONNECT does.
func TestProxySOCKS5(t *testing.T) {
	up := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "via socks") }))
	defer up.Close()
	p, src, _ := testProxy(t, nil)
	pu, _ := url.Parse("socks5://" + src.ID + ":" + src.Token + "@" + p.Addr())
	roots := x509.NewCertPool()
	roots.AddCert(p.CA.Cert)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu), TLSClientConfig: &tls.Config{RootCAs: roots}}}
	resp, err := client.Get(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "via socks" {
		t.Fatalf("%s", b)
	}
	if list := waitDone(t, p, 1); list[0].Source != src.ID {
		t.Fatalf("%+v", list)
	}
}

// Server-sent events are split into messages as they stream.
func TestProxySSE(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := range 3 {
			fmt.Fprintf(w, "event: tick\ndata: %d\n\n", i)
			w.(http.Flusher).Flush()
		}
	}))
	defer up.Close()
	p, _, client := testProxy(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", up.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	list := waitDone(t, p, 1)
	ex, _ := p.Store.Get(list[0].ID)
	if msgs := ex.Messages(0, 0); len(msgs) != 3 || msgs[2].Event != "tick" || string(msgs[2].Data) != "2" || list[0].Kind != proto.TrafficSSE {
		t.Fatalf("%+v %+v", list[0], msgs)
	}
}

// A reverse proxy in front of a dev server records what a browser loads
// from it, its HMR WebSocket too, the Host header as the browser sent it.
func TestReverseProxy(t *testing.T) {
	var hosts []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts = append(hosts, r.Host)
		io.WriteString(w, "<h1>dev</h1>")
	}))
	defer up.Close()
	p, src, _ := testProxy(t, nil)
	ln, err := p.Reverse(src, strings.TrimPrefix(up.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	req, _ := http.NewRequest("GET", "http://"+ln.Addr().String()+"/index.html", nil)
	req.Host = "localhost:5173"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "<h1>dev</h1>" || len(hosts) != 1 || hosts[0] != "localhost:5173" {
		t.Fatalf("%s %v", b, hosts)
	}
	if list := waitDone(t, p, 1); list[0].Path != "/index.html" || list[0].Source != src.ID {
		t.Fatalf("%+v", list)
	}
}
