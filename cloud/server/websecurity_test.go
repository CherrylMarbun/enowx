package server

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// Every form that changes something (/device, /signout, /invite,
// /invite/accept) refuses a post that isn't the cloud's own, even with the
// session cookie (as a browser ignoring SameSite sends it); a GET changes
// nothing; a browser's same-origin Origin alone still works.
func TestWebFormsRefuseOtherSites(t *testing.T) {
	e := newWebEnv(t, map[string]ghUser{"v": {ID: 1, Login: "v", Name: "V"}}, nil)
	ctx := t.Context()
	boss, _ := e.db.GitHubAccount(ctx, 2, "b", "B", "", "")
	team, _ := e.db.CreateTeam(ctx, boss.ID, "T")
	inv, _ := e.db.CreateInvite(ctx, team.ID, boss.ID, api.InviteParams{Role: api.RoleMember})
	k, _ := seal.NewDeviceKey()
	var dc api.DeviceCode
	send(t, "POST", e.srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "x", Kind: api.DeviceApp, PublicKey: k.Public()}, &dc)

	v := e.browser("v")
	e.signIn(v, "/")
	e.post(v, "/invite", url.Values{"token": {inv.Token}}, nil) // the invite cookie, as the page would
	sess := e.sessionCookie(v)
	if sess == "" {
		t.Fatal("not signed in")
	}
	attacks := []map[string]string{
		{"Sec-Fetch-Site": "cross-site", "Origin": e.srv.URL},
		{"Sec-Fetch-Site": "same-site"},
		{"Sec-Fetch-Site": "none"},
		{"Origin": "https://evil.example"},
		{"Origin": "null"},
		{"Origin": "null", "Referer": "https://evil.example/" + e.srv.URL + "/"},
		{"Referer": e.srv.URL + ".evil.example/device"},
		{"Origin": e.srv.URL + ".evil.example"},
		{"Origin": e.srv.URL + "@evil.example"},
		{"X-Nothing": "1"},
	}
	forms := map[string]url.Values{
		"/device":        {"code": {dc.UserCode}},
		"/signout":       nil,
		"/invite":        {"token": {inv.Token}},
		"/invite/accept": nil,
	}
	for path, form := range forms {
		for _, h := range attacks {
			if res, _ := e.post(v, path, form, h); res.StatusCode != http.StatusForbidden {
				t.Errorf("POST %s with %v: %d, want 403", path, h, res.StatusCode)
			}
		}
	}
	for _, p := range []string{"/device?code=" + dc.UserCode, "/invite", "/signout", "/invite/accept"} {
		e.get(v, p)
	}
	if _, err := e.db.WebSessionAccount(ctx, sess); err != nil {
		t.Errorf("signed out by another site's post")
	}
	if _, err := e.db.PendingDevice(ctx, dc.UserCode); err != nil {
		t.Errorf("code approved by another site's post or a GET")
	}
	if _, err := e.db.PeekInvite(ctx, inv.Token); err != nil {
		t.Errorf("invite used by another site's post or a GET")
	}
	if res, _ := e.post(v, "/device", forms["/device"], map[string]string{"Origin": e.srv.URL}); res.StatusCode != http.StatusOK {
		t.Errorf("same-origin Origin fallback: %d", res.StatusCode)
	}
}

// The state and session cookies are HttpOnly, SameSite=Lax and Secure
// behind an https URL; the session lasts 12 hours, is 256-bit random, kept
// only as its SHA-256, and dies on sign-out. A session cookie isn't a bearer
// token and a device token isn't a session.
func TestWebSessionCookies(t *testing.T) {
	h := New(Config{URL: "https://cloud.example", GitHubClientID: "cid", GitHubClientSecret: "s", GitHubURL: "https://gh.invalid"},
		storetest.Open(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", pathGitHub+"?next=/", nil))
	sc := rec.Header().Get("Set-Cookie")
	for _, want := range []string{"Secure", "HttpOnly", "SameSite=Lax", "Path=" + pathGitHub, "Max-Age=600"} {
		if !strings.Contains(sc, want) {
			t.Errorf("state cookie lacks %s: %s", want, sc)
		}
	}

	e := newWebEnv(t, map[string]ghUser{"v": {ID: 1, Login: "v", Name: "V"}}, nil)
	ctx := t.Context()
	v := noFollow(e.browser("v"))
	res, _ := v.Get(e.srv.URL + pathGitHub + "?next=/")
	res, _ = v.Get(res.Header.Get("Location"))
	res, _ = v.Get(res.Header.Get("Location")) // the callback
	cookies := res.Header.Values("Set-Cookie")
	sc = cookies[len(cookies)-1]
	for _, want := range []string{sessionCookie + "=", "HttpOnly", "SameSite=Lax", "Path=/", "Max-Age=43200"} {
		if !strings.Contains(sc, want) {
			t.Errorf("session cookie lacks %s: %s", want, sc)
		}
	}
	tok := e.sessionCookie(v)
	if len(tok) != 43 {
		t.Errorf("session token length %d", len(tok))
	}
	var raw, hashed int
	sum := sha256.Sum256([]byte(tok))
	e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM web_sessions WHERE hash = $1`, []byte(tok)).Scan(&raw)
	e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM web_sessions WHERE hash = $1`, sum[:]).Scan(&hashed)
	if raw != 0 || hashed != 1 {
		t.Errorf("session stored raw=%d hashed=%d", raw, hashed)
	}
	if code := call(t, e.srv.URL, "GET", api.PathMe, tok, nil); code != http.StatusUnauthorized {
		t.Errorf("session cookie as bearer: %d", code)
	}
	u, _ := url.Parse(e.srv.URL)
	withCookie := func(value string) *http.Client {
		b := e.browser("v")
		b.Jar.SetCookies(u, []*http.Cookie{{Name: sessionCookie, Value: value, Path: "/"}})
		return b
	}
	e.post(withCookie(tok), "/signout", nil, nil)
	if _, page := e.get(withCookie(tok), "/"); !strings.Contains(page, "Sign in with GitHub") {
		t.Errorf("old session cookie still signed in after sign-out")
	}

	acc, _ := e.db.GitHubAccount(ctx, 1, "v", "V", "", "")
	k, _ := seal.NewDeviceKey()
	_, devTok, _ := e.db.AddDevice(ctx, acc.ID, "d", api.DeviceApp, k.Public())
	if _, page := e.get(withCookie(devTok), "/"); !strings.Contains(page, "Sign in with GitHub") {
		t.Errorf("device token works as a web session")
	}
}

// Only "Bearer <token>" of a device that still exists opens the API: not a
// near miss, a device's ID, or a recovery key (a device with no token); no
// CORS header lets another site read answers.
func TestBearerTokensOnly(t *testing.T) {
	e := newWebEnv(t, nil, nil)
	ctx := t.Context()
	acc, _ := e.db.GitHubAccount(ctx, 1, "v", "V", "", "")
	k, _ := seal.NewDeviceKey()
	d, tok, _ := e.db.AddDevice(ctx, acc.ID, "d", api.DeviceApp, k.Public())
	rk, _ := seal.NewDeviceKey()
	rec, err := e.db.SetRecovery(ctx, acc.ID, rk.Public())
	if err != nil {
		t.Fatal(err)
	}
	for _, auth := range []string{"bearer " + tok, "Bearer  " + tok, "Basic " + tok, tok, "Bearer " + tok[:len(tok)-1], "Bearer " + d.ID, "Bearer " + rec.ID} {
		req, _ := http.NewRequest("GET", e.srv.URL+api.PathMe, nil)
		req.Header.Set("Authorization", auth)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("Authorization %q: %d", auth[:min(len(auth), 12)], res.StatusCode)
		}
	}
	req, _ := http.NewRequest("GET", e.srv.URL+api.PathMe, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Access-Control-Allow-Origin") != "" || res.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("CORS: %d %v", res.StatusCode, res.Header)
	}
	if code, _ := send(t, "DELETE", e.srv.URL+"/v1/devices/"+d.ID, tok, nil, nil); code != http.StatusNoContent {
		t.Fatalf("remove: %d", code)
	}
	if code := call(t, e.srv.URL, "GET", api.PathMe, tok, nil); code != http.StatusUnauthorized {
		t.Errorf("removed device's token: %d", code)
	}
}

// Every answer is neither sniffed nor cached (the logo keeps its own
// caching); behind an https URL it also asks for HTTPS only, and plain
// HTTP the proxy passes on is sent to HTTPS.
func TestEveryAnswerCarriesSecurityHeaders(t *testing.T) {
	secure := start(t, Config{URL: "https://cloud.example"})
	plain := start(t, Config{})
	get := func(base, path string, hdr ...string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest("GET", base+path, nil)
		for i := 0; i+1 < len(hdr); i += 2 {
			req.Header.Set(hdr[i], hdr[i+1])
		}
		res, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res
	}
	for _, base := range []string{secure.URL, plain.URL} {
		for _, path := range []string{api.PathHealth, api.PathInfo, api.PathMe, "/", "/device", "/invite", "/nowhere"} {
			res := get(base, path)
			if res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Cache-Control") != "no-store" {
				t.Errorf("%s: %d nosniff=%q cache=%q", path, res.StatusCode, res.Header.Get("X-Content-Type-Options"), res.Header.Get("Cache-Control"))
			}
			if hsts := res.Header.Get("Strict-Transport-Security"); (hsts != "") != (base == secure.URL) {
				t.Errorf("%s at %s: HSTS %q", path, base, hsts)
			}
		}
		if res := get(base, "/logo.svg"); !strings.Contains(res.Header.Get("Cache-Control"), "max-age") || res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("logo: %v", res.Header)
		}
	}
	res := get(secure.URL, api.PathInfo+"?x=1", "X-Forwarded-Proto", "http")
	if res.StatusCode != http.StatusPermanentRedirect || res.Header.Get("Location") != "https://cloud.example"+api.PathInfo+"?x=1" {
		t.Errorf("plain HTTP behind the proxy: %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	if res := get(secure.URL, api.PathInfo, "X-Forwarded-Proto", "https"); res.StatusCode != http.StatusOK {
		t.Errorf("HTTPS behind the proxy: %d", res.StatusCode)
	}
	if res := get(plain.URL, api.PathInfo, "X-Forwarded-Proto", "http"); res.StatusCode != http.StatusOK {
		t.Errorf("a cloud without an https URL redirected: %d", res.StatusCode)
	}
}

// Hostile names (team, inviter, device, GitHub name and login) are escaped
// in every page; a javascript: avatar is neutralized; the invite page's
// script is exactly the one its CSP hash allows, and other pages allow no
// script.
func TestWebPagesEscapeNames(t *testing.T) {
	xss := `"><img src=x onerror=alert(1)><script>alert(1)</script>`
	e := newWebEnv(t, map[string]ghUser{"v": {ID: 1, Login: "v" + xss, Name: xss, Avatar: "javascript:alert(1)"}}, nil)
	ctx := t.Context()
	boss, _ := e.db.GitHubAccount(ctx, 2, "b", xss, "", "")
	team, _ := e.db.CreateTeam(ctx, boss.ID, xss)
	inv, _ := e.db.CreateInvite(ctx, team.ID, boss.ID, api.InviteParams{Role: api.RoleMember})
	k, _ := seal.NewDeviceKey()
	var dc api.DeviceCode
	send(t, "POST", e.srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: xss, Kind: api.DeviceApp, PublicKey: k.Public()}, &dc)

	v := e.browser("v")
	bad := func(where, page string) {
		t.Helper()
		for _, s := range []string{"<img src=x", "<script>alert", `src="javascript:`} {
			if strings.Contains(page, s) {
				t.Errorf("%s: unescaped %q", where, s)
			}
		}
	}
	res, page := e.get(v, "/invite")
	csp := res.Header.Get("Content-Security-Policy")
	i, j := strings.Index(page, "<script>"), strings.Index(page, "</script>")
	if i < 0 || j < i {
		t.Fatalf("no script on the invite page: %s", page)
	}
	h := sha256.Sum256([]byte(page[i+len("<script>") : j]))
	if !strings.Contains(csp, "'sha256-"+base64.StdEncoding.EncodeToString(h[:])+"'") {
		t.Errorf("invite script doesn't match CSP hash: %s", csp)
	}
	_, page = e.post(v, "/invite", url.Values{"token": {inv.Token}}, nil)
	bad("invite signed out", page)
	_, page = e.signIn(v, "/invite")
	bad("invite signed in", page)
	_, page = e.get(v, "/device?code="+url.QueryEscape(dc.UserCode))
	bad("device", page)
	_, page = e.get(v, "/device?code="+url.QueryEscape(xss))
	bad("device bad code", page)
	_, page = e.post(v, "/device", url.Values{"code": {dc.UserCode}}, nil)
	bad("approved", page)
	_, page = e.post(v, "/invite/accept", nil, nil)
	bad("joined", page)
	res, _ = e.get(v, "/")
	if csp := res.Header.Get("Content-Security-Policy"); strings.Contains(csp, "script-src") || !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("home CSP: %s", csp)
	}
}
