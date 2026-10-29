package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// Each door open without a token takes its number of requests a minute
// from one address, then answers 429; another address isn't held back.
func TestDoorsLimitEachAddress(t *testing.T) {
	srv := start(t, Config{GitHubClientID: "cid", GitHubClientSecret: "s", GitHubURL: "https://gh.invalid"})
	doors := map[string]int{
		"POST " + api.PathDeviceCode:  10,
		"POST " + api.PathDeviceToken: 40,
		"POST " + api.PathInvitePeek:  20,
		"GET /device":                 30,
		"POST /device":                20,
		"POST /invite":                20,
		"POST /invite/accept":         20,
		"GET " + pathGitHub:           20,
	}
	knock := func(door string, hdr ...string) int {
		t.Helper()
		method, path, _ := strings.Cut(door, " ")
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader("{}"))
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	for door, max := range doors {
		addr := "CF-Connecting-IP"
		for i := range max {
			if code := knock(door, addr, "198.51.100.1"); code == http.StatusTooManyRequests {
				t.Fatalf("%s: refused at %d of %d", door, i+1, max)
			}
		}
		if code := knock(door, addr, "198.51.100.1"); code != http.StatusTooManyRequests {
			t.Errorf("%s: %d over the limit", door, code)
		}
		if code := knock(door, addr, "198.51.100.2"); code == http.StatusTooManyRequests {
			t.Errorf("%s: another address refused", door)
		}
	}

	// Cloudflare's address first, then X-Forwarded-For's first, then the
	// connection's.
	door := "POST " + api.PathDeviceCode
	if code := knock(door, "CF-Connecting-IP", "198.51.100.1", "X-Forwarded-For", "203.0.113.1"); code != http.StatusTooManyRequests {
		t.Errorf("X-Forwarded-For over Cloudflare's address: %d", code)
	}
	for range 10 {
		knock(door, "X-Forwarded-For", "203.0.113.1, 10.0.0.1")
	}
	if code := knock(door, "X-Forwarded-For", "203.0.113.1"); code != http.StatusTooManyRequests {
		t.Errorf("X-Forwarded-For's first address: %d", code)
	}
	for range 10 {
		knock(door)
	}
	if code := knock(door); code != http.StatusTooManyRequests {
		t.Errorf("the connection's address: %d", code)
	}
	if code := knock(door, "X-Forwarded-For", "203.0.113.2"); code == http.StatusTooManyRequests {
		t.Errorf("another forwarded address refused: %d", code)
	}
}

// A member's account records 120 audit actions and gets 30 host tickets a
// minute, then waits.
func TestAccountsLimitAuditAndTickets(t *testing.T) {
	e := newTeamEnv(t, Config{})
	ada := e.signIn(1, "ada", "")
	team := e.team(ada, "t")
	k, _ := seal.NewDeviceKey()
	host, _, err := e.db.AddDevice(t.Context(), ada.acc.ID, "server", api.DeviceHost, k.Public())
	if err != nil {
		t.Fatal(err)
	}
	limited := func(what string, max int, do func() (int, string)) {
		t.Helper()
		for i := range max {
			if code, msg := do(); code == http.StatusTooManyRequests || code >= 400 {
				t.Fatalf("%s %d of %d: %d %s", what, i+1, max, code, msg)
			}
		}
		if code, _ := do(); code != http.StatusTooManyRequests {
			t.Errorf("%s over the limit: %d", what, code)
		}
	}
	n := 0
	limited("audit", 120, func() (int, string) {
		n++
		return send(t, "POST", e.url+"/v1/teams/"+team+"/audit", ada.token, api.AuditParams{Action: api.AuditRulesChanged, Detail: fmt.Sprint(n)}, nil)
	})
	limited("ticket", 30, func() (int, string) {
		return send(t, "POST", e.url+"/v1/teams/"+team+"/tickets", ada.token, api.HostTicketParams{Host: host.ID}, nil)
	})
}
