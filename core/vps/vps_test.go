package vps

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const userKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB4Zgz3K me@laptop"

// fakeAPI answers a provider's paths with canned JSON, the way its
// documentation shows them, and records what was sent.
type fakeAPI struct {
	t      *testing.T
	routes map[string]string // "METHOD /path" → JSON answer
	mu     sync.Mutex
	got    map[string]map[string]any // bodies sent, by route
	polls  int
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer tok" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"unauthorized"}}`)
		return
	}
	route := r.Method + " " + r.URL.Path
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Body != nil {
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			f.got[route] = body
		}
	}
	answer, ok := f.routes[route]
	if !ok {
		f.t.Errorf("unexpected %s", route)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if strings.HasPrefix(route, "GET /servers/") || strings.HasPrefix(route, "GET /droplets/") || strings.HasPrefix(route, "GET /instances/") {
		// Booting, then up.
		f.polls++
		if f.polls == 1 {
			answer = strings.NewReplacer(`"running"`, `"initializing"`, `"active"`, `"new"`).Replace(answer)
		}
	}
	if r.Method == "DELETE" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_, _ = io.WriteString(w, answer)
}

func serve(t *testing.T, routes map[string]string) (*fakeAPI, string) {
	f := &fakeAPI{t: t, routes: routes, got: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv.URL
}

// create runs the whole of creating a server with c: regions, sizes, the
// server itself, then waiting until it is up.
func create(t *testing.T, c Client, region string) ([]Option, Server) {
	t.Helper()
	ctx := context.Background()
	regions, err := c.Regions(ctx)
	if err != nil || len(regions) == 0 {
		t.Fatalf("regions %+v, %v", regions, err)
	}
	sizes, err := c.Sizes(ctx, region)
	if err != nil || len(sizes) == 0 {
		t.Fatalf("sizes %+v, %v", sizes, err)
	}
	s, err := c.Create(ctx, CreateParams{Name: "enowx-1", Region: region, Size: sizes[0].ID, PublicKey: userKey})
	if err != nil {
		t.Fatal(err)
	}
	up, err := WaitReady(ctx, c, s.ID, time.Millisecond, nil)
	if err != nil || !up.Ready || up.IPv4 != "203.0.113.7" {
		t.Fatalf("up %+v, %v", up, err)
	}
	if err := c.Delete(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	return sizes, up
}

func TestHetzner(t *testing.T) {
	f, base := serve(t, map[string]string{
		"GET /locations": `{"locations":[{"name":"fsn1","description":"Falkenstein DC Park 1","city":"Falkenstein","country":"DE"}]}`,
		"GET /server_types": `{"server_types":[
			{"name":"cx32","cores":4,"memory":8,"disk":80,"architecture":"x86","deprecated":false,"prices":[{"location":"fsn1","price_monthly":{"gross":"7.4900"}}]},
			{"name":"cx22","cores":2,"memory":4,"disk":40,"architecture":"x86","deprecated":false,"prices":[{"location":"fsn1","price_monthly":{"gross":"4.5900"}}]},
			{"name":"cx11","cores":1,"memory":2,"disk":20,"architecture":"x86","deprecated":true,"prices":[{"location":"fsn1","price_monthly":{"gross":"3.0000"}}]},
			{"name":"cpx11","cores":2,"memory":2,"disk":40,"architecture":"x86","deprecated":false,"prices":[{"location":"ash","price_monthly":{"gross":"5.0000"}}]}]}`,
		"GET /ssh_keys":     `{"ssh_keys":[{"id":42,"name":"enowX old","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB4Zgz3K other-comment"}]}`,
		"POST /servers":     `{"server":{"id":7,"name":"enowx-1","status":"initializing","public_net":{"ipv4":{"ip":"203.0.113.7"}}}}`,
		"GET /servers/7":    `{"server":{"id":7,"name":"enowx-1","status":"running","public_net":{"ipv4":{"ip":"203.0.113.7"}}}}`,
		"DELETE /servers/7": ``,
	})
	c, err := New("hetzner", "tok", base)
	if err != nil {
		t.Fatal(err)
	}
	sizes, _ := create(t, c, "fsn1")
	// Cheapest first, without deprecated types or those not in the region.
	if len(sizes) != 2 || sizes[0].ID != "cx22" || sizes[0].Monthly != 4.59 || !strings.Contains(sizes[0].Detail, "2 vCPU · 4 GB") {
		t.Fatalf("sizes %+v", sizes)
	}
	body := f.got["POST /servers"]
	if body["image"] != "ubuntu-24.04" || body["location"] != "fsn1" || body["server_type"] != "cx22" {
		t.Fatalf("created with %v", body)
	}
	// The key there already, under another comment, was used again.
	if keys := body["ssh_keys"].([]any); len(keys) != 1 || keys[0].(float64) != 42 {
		t.Fatalf("ssh keys %v", body["ssh_keys"])
	}
	if _, made := f.got["POST /ssh_keys"]; made {
		t.Fatal("added the key twice")
	}
}

func TestDigitalOcean(t *testing.T) {
	f, base := serve(t, map[string]string{
		"GET /regions":        `{"regions":[{"slug":"sgp1","name":"Singapore 1","available":true},{"slug":"old1","name":"Old","available":false}]}`,
		"GET /sizes":          `{"sizes":[{"slug":"s-1vcpu-1gb","memory":1024,"vcpus":1,"disk":25,"price_monthly":6,"regions":["sgp1"],"available":true}]}`,
		"GET /account/keys":   `{"ssh_keys":[]}`,
		"POST /account/keys":  `{"ssh_key":{"id":512,"name":"enowX"}}`,
		"POST /droplets":      `{"droplet":{"id":99,"name":"enowx-1","status":"new","networks":{"v4":[]}}}`,
		"GET /droplets/99":    `{"droplet":{"id":99,"name":"enowx-1","status":"active","networks":{"v4":[{"ip_address":"10.0.0.2","type":"private"},{"ip_address":"203.0.113.7","type":"public"}]}}}`,
		"DELETE /droplets/99": ``,
	})
	c, err := New("digitalocean", "tok", base)
	if err != nil {
		t.Fatal(err)
	}
	sizes, _ := create(t, c, "sgp1")
	if sizes[0].Detail != "1 vCPU · 1 GB · 25 GB disk" {
		t.Fatalf("sizes %+v", sizes)
	}
	if k := f.got["POST /account/keys"]; k["public_key"] != userKey {
		t.Fatalf("key sent %v", k)
	}
	body := f.got["POST /droplets"]
	if body["image"] != "ubuntu-24-04-x64" || body["region"] != "sgp1" || body["ssh_keys"].([]any)[0].(float64) != 512 {
		t.Fatalf("created with %v", body)
	}
}

func TestVultr(t *testing.T) {
	f, base := serve(t, map[string]string{
		"GET /regions":             `{"regions":[{"id":"sgp","city":"Singapore","country":"SG","continent":"Asia"}]}`,
		"GET /plans":               `{"plans":[{"id":"vc2-1c-1gb","vcpu_count":1,"ram":1024,"disk":25,"monthly_cost":5,"locations":["sgp"]}]}`,
		"GET /ssh-keys":            `{"ssh_keys":[]}`,
		"POST /ssh-keys":           `{"ssh_key":{"id":"key-1","name":"enowX"}}`,
		"GET /os":                  `{"os":[{"id":2136,"name":"Ubuntu 22.04 LTS x64","arch":"x64"},{"id":2284,"name":"Ubuntu 24.04 LTS x64","arch":"x64"},{"id":2290,"name":"Ubuntu 24.04 LTS arm64","arch":"arm64"}]}`,
		"POST /instances":          `{"instance":{"id":"inst-1","label":"enowx-1","main_ip":"0.0.0.0","status":"pending","power_status":"stopped"}}`,
		"GET /instances/inst-1":    `{"instance":{"id":"inst-1","label":"enowx-1","main_ip":"203.0.113.7","status":"active","power_status":"running","server_status":"ok"}}`,
		"DELETE /instances/inst-1": ``,
	})
	c, err := New("vultr", "tok", base)
	if err != nil {
		t.Fatal(err)
	}
	create(t, c, "sgp")
	body := f.got["POST /instances"]
	if body["os_id"].(float64) != 2284 || body["plan"] != "vc2-1c-1gb" || body["sshkey_id"].([]any)[0] != "key-1" {
		t.Fatalf("created with %v", body)
	}
}

func TestATokenRefusedSaysSo(t *testing.T) {
	_, base := serve(t, map[string]string{})
	c, _ := New("hetzner", "wrong", base)
	if _, err := c.Regions(context.Background()); err == nil || !strings.Contains(err.Error(), "refused the token") {
		t.Fatalf("err = %v", err)
	}
	if _, err := New("hetzner", " ", ""); err == nil {
		t.Fatal("an empty token was taken")
	}
}
