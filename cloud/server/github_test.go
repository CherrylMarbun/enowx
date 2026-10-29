package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// fakeGitHub signs anyone in as ada, whose profile hides her email.
func fakeGitHub(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != "cid" || !strings.Contains(q.Get("scope"), "user:email") {
			http.Error(w, "bad client", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=good&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if r.PostFormValue("code") != "good" || r.PostFormValue("client_secret") != "csecret" {
			json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "gho_ada"})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gho_ada" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 42, "login": "ada", "name": "Ada Lovelace", "avatar_url": "https://example.com/a.png"})
	})
	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{"email": "old@example.com", "primary": false, "verified": true}, {"email": "ada@example.com", "primary": true, "verified": true}})
	})
	gh := httptest.NewServer(mux)
	t.Cleanup(gh.Close)
	return gh
}

// The app signs in through the browser: its code's page sends the browser
// to GitHub and back, shows what asks, and the app gets its token once
// approved. A sign-in that didn't start here, a way out to another site,
// and an approval posted from elsewhere are refused.
func TestAppSignsInWithGitHub(t *testing.T) {
	gh := fakeGitHub(t)
	var handler http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	db := storetest.Open(t)
	handler = New(Config{URL: srv.URL, GitHubClientID: "cid", GitHubClientSecret: "csecret", GitHubURL: gh.URL, GitHubAPIURL: gh.URL},
		db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var in api.Info
	call(t, srv.URL, "GET", api.PathInfo, "", &in)
	if len(in.SignIn) != 1 || in.SignIn[0] != api.SignInGitHub {
		t.Fatalf("sign-in methods: %v", in.SignIn)
	}
	key, _ := seal.NewDeviceKey()
	var dc api.DeviceCode
	send(t, "POST", srv.URL+api.PathDeviceCode, "", api.DeviceCodeParams{Name: "Ada's Mac", Kind: api.DeviceApp, PublicKey: key.Public()}, &dc)
	if dc.VerifyURLComplete != srv.URL+"/device?code="+dc.UserCode {
		t.Fatalf("verify URL: %+v", dc)
	}

	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar}
	read := func(res *http.Response) string {
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return string(b)
	}
	res, err := browser.Get(dc.VerifyURLComplete)
	if err != nil {
		t.Fatal(err)
	}
	page := read(res)
	if res.Request.URL.Path != "/device" || !strings.Contains(page, "Ada&#39;s Mac") || !strings.Contains(page, "Approve") || !strings.Contains(page, "@ada") {
		t.Fatalf("after signing in, at %s: %s", res.Request.URL, page)
	}
	var accID string
	db.QueryRowContext(t.Context(), `SELECT id FROM accounts WHERE github_id = $1`, 42).Scan(&accID)
	acc, _ := db.Account(t.Context(), accID)
	if acc.Email != "ada@example.com" || acc.GitHub != "ada" {
		t.Fatalf("account: %+v", acc)
	}

	approve := func(origin string) (int, string) {
		req, _ := http.NewRequest("POST", srv.URL+"/device", strings.NewReader(url.Values{"code": {dc.UserCode}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := browser.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, read(res)
	}
	if code, _ := approve("https://evil.example"); code != http.StatusForbidden {
		t.Fatalf("approval from another site: %d", code)
	}
	if code, body := approve(srv.URL); code != http.StatusOK || !strings.Contains(body, "is signed in to your account") {
		t.Fatalf("approve: %d %s", code, body)
	}
	var signed api.SignedIn
	if code, msg := send(t, "POST", srv.URL+api.PathDeviceToken, "", api.DeviceTokenParams{DeviceCode: dc.DeviceCode}, &signed); code != http.StatusOK || signed.Me.Account.GitHub != "ada" {
		t.Fatalf("the app's token: %d %s %+v", code, msg, signed)
	}

	// A callback whose state isn't the browser's is refused.
	res, _ = browser.Get(srv.URL + pathGitHubCallback + "?code=good&state=forged")
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("forged state: %d", res.StatusCode)
	}
	read(res)
	// Signing in never leaves for another site.
	noFollow := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, _ = noFollow.Get(srv.URL + pathGitHub + "?next=//evil.example/x")
	res.Body.Close()
	gh1, _ := url.Parse(res.Header.Get("Location"))
	res, _ = noFollow.Get(gh.URL + "/login/oauth/authorize?" + gh1.RawQuery)
	res.Body.Close()
	res, _ = noFollow.Get(res.Header.Get("Location"))
	res.Body.Close()
	if loc := res.Header.Get("Location"); loc != "/" {
		t.Fatalf("after signing in with next=//evil.example: %q", loc)
	}
}

