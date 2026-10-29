package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// A device's sign-in sent to someone else to approve stands out: the
// approval page and the pending API say when and where it started, the
// page warns when that isn't the browser's network, and its name can't be
// dressed up with control characters or made to fill the page.
func TestDeviceApprovalShowsWhereItStarted(t *testing.T) {
	e := newWebEnv(t, map[string]ghUser{"victim": {ID: 100, Login: "victim", Name: "Vic"}}, nil)
	key, _ := seal.NewDeviceKey()
	lure := "Vic's MacBook\u202e\u200b\x07 (this computer) " + strings.Repeat("verified ", 300)
	from := map[string]string{"CF-Connecting-IP": "203.0.113.7", "CF-IPCountry": "NL"}
	var dc api.DeviceCode
	if code, msg := sendWith(t, "POST", e.srv.URL+api.PathDeviceCode, "", from, api.DeviceCodeParams{Name: lure, Kind: api.DeviceApp, PublicKey: key.Public()}, &dc); code != http.StatusOK {
		t.Fatalf("start: %d %s", code, msg)
	}

	victim := e.browser("victim")
	u, _ := url.Parse(dc.VerifyURLComplete)
	res, page := e.get(victim, u.RequestURI())
	if res.Request.URL.Path != "/device" || !strings.Contains(page, "Approve this device?") {
		t.Fatalf("approval page: %s %s", res.Request.URL, page)
	}
	if !strings.Contains(page, "Started ") || !strings.Contains(page, " ago from 203.0.113.7 (NL)") {
		t.Errorf("the page doesn't say when and where the sign-in started: %s", page)
	}
	if !strings.Contains(page, "started from another network than this browser") {
		t.Errorf("no warning for a sign-in started elsewhere: %s", page)
	}
	for _, c := range []string{"\u202e", "\u200b", "\x07", strings.Repeat("verified ", 10)} {
		if strings.Contains(page, c) {
			t.Errorf("the name keeps %q", c)
		}
	}

	ctx := t.Context()
	acc, _ := e.db.GitHubAccount(ctx, 100, "victim", "Vic", "", "")
	_, tok, _ := e.db.AddDevice(ctx, acc.ID, "Vic's Mac", api.DeviceApp, key.Public())
	pendingPath := e.srv.URL + "/v1/auth/device/pending/" + dc.UserCode
	var p api.PendingDevice
	if code, msg := sendWith(t, "GET", pendingPath, tok, map[string]string{"CF-Connecting-IP": "198.51.100.1"}, nil, &p); code != http.StatusOK {
		t.Fatalf("pending: %d %s", code, msg)
	}
	if p.From != "203.0.113.7" || p.Place != "NL" || p.SameNetwork {
		t.Errorf("pending, asked from elsewhere: %+v", p)
	}
	if n := utf8.RuneCountInString(p.Name); n == 0 || n > 64 || strings.ContainsAny(p.Name, "\u202e\u200b\x07") {
		t.Errorf("name kept: %q", p.Name)
	}
	p = api.PendingDevice{}
	sendWith(t, "GET", pendingPath, tok, map[string]string{"CF-Connecting-IP": "203.0.113.7"}, nil, &p)
	if !p.SameNetwork {
		t.Errorf("pending, asked from the same network: %+v", p)
	}

	// Started from the browser's own network: no warning.
	var mine api.DeviceCode
	send(t, "POST", e.srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "mine", Kind: api.DeviceApp, PublicKey: key.Public()}, &mine)
	if _, page := e.get(victim, "/device?code="+url.QueryEscape(mine.UserCode)); strings.Contains(page, "another network") || !strings.Contains(page, "from 127.0.0.1") {
		t.Errorf("a sign-in from the browser's own network: %s", page)
	}
}

// A code approved but claimed after it expired gives no token; the user
// code shown on screen isn't the device code that claims.
func TestApprovedCodeExpires(t *testing.T) {
	srv, db := startWithDB(t, Config{})
	ctx := t.Context()
	acc, _ := db.GitHubAccount(ctx, 1, "ada", "Ada", "", "")
	k, _ := seal.NewDeviceKey()
	_, tok, _ := db.AddDevice(ctx, acc.ID, "Ada's Mac", api.DeviceApp, k.Public())
	var dc api.DeviceCode
	send(t, "POST", srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "h", Kind: api.DeviceHost, PublicKey: k.Public()}, &dc)
	if code, msg := send(t, "POST", srv.URL+api.PathDeviceToken, "", api.DeviceTokenParams{DeviceCode: dc.UserCode}, nil); code != http.StatusBadRequest || msg != api.ErrExpired {
		t.Fatalf("the user code as the device code: %d %s", code, msg)
	}
	if code, msg := send(t, "POST", srv.URL+api.PathDeviceApprove, tok, api.DeviceApproveParams{UserCode: dc.UserCode}, nil); code != http.StatusNoContent {
		t.Fatalf("approve: %d %s", code, msg)
	}
	db.ExecContext(ctx, `UPDATE device_codes SET expires = '2000-01-01T00:00:00Z'`)
	if code, msg := send(t, "POST", srv.URL+api.PathDeviceToken, "", api.DeviceTokenParams{DeviceCode: dc.DeviceCode}, nil); code != http.StatusBadRequest || msg != api.ErrExpired {
		t.Fatalf("an approved code claimed after it expired: %d %s", code, msg)
	}
}
