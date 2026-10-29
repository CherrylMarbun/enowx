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

	"github.com/enowx/enowx/cloud/store"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// ghUser is a GitHub user the fake GitHub of webEnv knows.
type ghUser struct {
	ID     int64  `json:"id"`
	Login  string `json:"login"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Avatar string `json:"avatar_url"`
}

// webEnv is a cloud with GitHub sign-in, against a fake GitHub that signs
// in whichever user the browser's own GitHub cookie names: several people
// in their own browsers.
type webEnv struct {
	t   *testing.T
	gh  *httptest.Server
	srv *httptest.Server
	db  *store.DB
}

// newWebEnv starts a cloud whose GitHub knows users (by login); mod, when
// given, changes its Config.
func newWebEnv(t *testing.T, users map[string]ghUser, mod func(*Config)) *webEnv {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("gh_user")
		if err != nil {
			http.Error(w, "not signed in to github", http.StatusUnauthorized)
			return
		}
		q := r.URL.Query()
		http.Redirect(w, r, q.Get("redirect_uri")+"?code=c_"+c.Value+"&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("POST /login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		login, ok := strings.CutPrefix(r.PostFormValue("code"), "c_")
		if _, known := users[login]; !ok || !known {
			json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "t_" + login})
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		login, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer t_")
		u, ok := users[login]
		if !ok {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(u)
	})
	mux.HandleFunc("GET /user/emails", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{})
	})
	gh := httptest.NewServer(mux)
	t.Cleanup(gh.Close)
	var handler http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	db := storetest.Open(t)
	cfg := Config{URL: srv.URL, GitHubClientID: "cid", GitHubClientSecret: "csecret", GitHubURL: gh.URL, GitHubAPIURL: gh.URL}
	if mod != nil {
		mod(&cfg)
	}
	handler = New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &webEnv{t: t, gh: gh, srv: srv, db: db}
}

// browser is a browser signed in to GitHub (not yet to the cloud) as login.
func (e *webEnv) browser(login string) *http.Client {
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(e.gh.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "gh_user", Value: login, Path: "/"}})
	return &http.Client{Jar: jar}
}

// noFollow is b, stopping at each redirect.
func noFollow(b *http.Client) *http.Client {
	return &http.Client{Jar: b.Jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func readPage(t *testing.T, res *http.Response, err error) (*http.Response, string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

// get fetches path in browser b, following redirects.
func (e *webEnv) get(b *http.Client, path string) (*http.Response, string) {
	e.t.Helper()
	return e.getAbs(b, e.srv.URL+path)
}

// getAbs fetches absolute URL u in browser b, following redirects.
func (e *webEnv) getAbs(b *http.Client, u string) (*http.Response, string) {
	e.t.Helper()
	res, err := b.Get(u)
	return readPage(e.t, res, err)
}

// do asks path in browser b with form and headers hdr (none: the
// same-origin form a page of the cloud's posts).
func (e *webEnv) do(b *http.Client, method, path string, form url.Values, hdr map[string]string) (*http.Response, string) {
	e.t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, strings.NewReader(form.Encode()))
	if method == "POST" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if hdr == nil {
		hdr = map[string]string{"Sec-Fetch-Site": "same-origin"}
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := b.Do(req)
	return readPage(e.t, res, err)
}

// post posts form to path in browser b; see do.
func (e *webEnv) post(b *http.Client, path string, form url.Values, hdr map[string]string) (*http.Response, string) {
	e.t.Helper()
	return e.do(b, "POST", path, form, hdr)
}

// signIn signs browser b in to the cloud through GitHub, landing on next.
func (e *webEnv) signIn(b *http.Client, next string) (*http.Response, string) {
	e.t.Helper()
	return e.get(b, pathGitHub+"?next="+url.QueryEscape(next))
}

// sessionCookie is browser b's session on the cloud, if any.
func (e *webEnv) sessionCookie(b *http.Client) string {
	u, _ := url.Parse(e.srv.URL)
	for _, c := range b.Jar.Cookies(u) {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	return ""
}
