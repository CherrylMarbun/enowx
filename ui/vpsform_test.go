package ui

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/theme"
)

// Create a server: the token reads the provider's regions and sizes, and
// creating waits until the server and its SSH are up, then adds it to the
// hosts with what deletes it later.
func TestCreateAServerAddsItAsAHost(t *testing.T) {
	routes := map[string]string{
		"GET /locations":    `{"locations":[{"name":"fsn1","city":"Falkenstein","country":"DE"}]}`,
		"GET /server_types": `{"server_types":[{"name":"cx22","cores":2,"memory":4,"disk":40,"architecture":"x86","prices":[{"location":"fsn1","price_monthly":{"gross":"4.59"}}]}]}`,
		"GET /ssh_keys":     `{"ssh_keys":[]}`,
		"POST /ssh_keys":    `{"ssh_key":{"id":5}}`,
		"POST /servers":     `{"server":{"id":7,"name":"box","status":"initializing","public_net":{"ipv4":{"ip":"127.0.0.1"}}}}`,
		"GET /servers/7":    `{"server":{"id":7,"name":"box","status":"running","public_net":{"ipv4":{"ip":"127.0.0.1"}}}}`,
	}
	created := ""
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == "POST" && r.URL.Path == "/servers" {
			body, _ := io.ReadAll(r.Body)
			created = string(body)
		}
		_, _ = io.WriteString(w, routes[r.Method+" "+r.URL.Path])
	}))
	defer api.Close()
	ssh, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ssh.Close()
	go func() {
		for {
			c, err := ssh.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	vpsAPI, sshPort = api.URL, strings.Split(ssh.Addr().String(), ":")[1]
	defer func() { vpsAPI, sshPort = "", "22" }()

	dir := t.TempDir()
	key := filepath.Join(dir, "id_ed25519.pub")
	if err := os.WriteFile(key, []byte("ssh-ed25519 AAAAC3Nza me@laptop\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := hosts.Open(filepath.Join(dir, "hosts.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := New(Options{Hosts: store})
	posted := make(chan func(), 64)
	a.onMain = func(fn func()) { posted <- fn }
	frame := func(tt *ui.Tester) {
		for {
			select {
			case fn := <-posted:
				fn()
				continue
			default:
			}
			break
		}
		tt.Frame()
	}
	a.creating = &vpsForm{open: true, provider: "hetzner", token: "tok", name: "box", key: key}
	f := a.creating
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(640).Height(760).Background(theme.Background).Children(func() { a.createServerDialog(c) })
	}, 640, 760)
	a.loadRegions(f)
	deadline := time.Now().Add(10 * time.Second)
	for f.size == "" {
		if time.Now().After(deadline) {
			t.Fatalf("no sizes: %+v", f)
		}
		frame(tt)
		time.Sleep(20 * time.Millisecond)
	}
	frame(tt)
	if !tt.HasText("Region") || !tt.HasText("Create") {
		t.Fatalf("form: %q", tt.Texts())
	}
	shot(t, tt, "create-server.png")
	if err := tt.Click("Create"); err != nil {
		t.Fatal(err)
	}
	for f.open {
		if time.Now().After(deadline.Add(20 * time.Second)) {
			t.Fatalf("never created: %+v", f)
		}
		frame(tt)
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(created, `"image":"ubuntu-24.04"`) || !strings.Contains(created, `"ssh_keys":[5]`) {
		t.Fatalf("created with %s", created)
	}
	list := store.List()
	if len(list) != 1 || list[0].Address != "root@127.0.0.1" || list[0].Provider != "hetzner" || list[0].ProviderID != "7" {
		t.Fatalf("hosts %+v", list)
	}
}
