package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// next= never sends the browser off the cloud after signing in, whatever
// the trick (scheme, //, backslashes, dot segments, controls).
func TestSignInStaysOnTheCloud(t *testing.T) {
	e := newWebEnv(t, map[string]ghUser{"v": {ID: 1, Login: "v", Name: "V"}}, nil)
	srvURL, _ := url.Parse(e.srv.URL)
	for _, next := range []string{
		"https://evil.example/", "//evil.example/", "/\\evil.example", "/./\\evil.example", "/.//evil.example",
		"/../\\evil.example", "/\t/evil.example", "/%2F%2Fevil.example", "/;//evil.example", "///evil.example",
		"/\\/evil.example", "\\\\evil.example", "/device?code=x|//evil.example", " //evil.example", "/x/..//evil.example",
	} {
		b := noFollow(e.browser("v"))
		loc := e.srv.URL + pathGitHub + "?next=" + url.QueryEscape(next)
		var last string
		signedIn := false
		for range 5 {
			res, err := b.Get(loc)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusFound && res.StatusCode != http.StatusSeeOther {
				break
			}
			last = res.Header.Get("Location")
			u, _ := res.Request.URL.Parse(last)
			if u.Host != srvURL.Host && !strings.HasPrefix(u.String(), e.gh.URL) {
				t.Errorf("next=%q: redirected off-site to %q", next, last)
			}
			loc = u.String()
			if res.Request.URL.Path == pathGitHubCallback {
				signedIn = true
				break
			}
		}
		if !signedIn {
			t.Errorf("next=%q: sign-in never came back from the callback", next)
		}
		// What a browser makes of the final Location: backslashes are slashes.
		if bu, _ := srvURL.Parse(strings.ReplaceAll(last, "\\", "/")); bu == nil || bu.Host != srvURL.Host {
			t.Errorf("next=%q: final Location %q leaves the cloud in a browser", next, last)
		}
	}
}

// The OAuth state is bound to the browser's cookie: a callback URL carrying
// someone else's code and state, opened in the victim's browser, signs it
// in to nothing, whether or not the victim has a sign-in of their own
// pending; GitHub's error text needs a valid state first.
func TestSignInCallbackNeedsItsBrowser(t *testing.T) {
	e := newWebEnv(t, map[string]ghUser{"m": {ID: 666, Login: "m", Name: "M"}, "v": {ID: 1, Login: "v", Name: "V"}}, nil)
	m := noFollow(e.browser("m"))
	res, _ := m.Get(e.srv.URL + pathGitHub + "?next=/")
	res.Body.Close()
	res, _ = m.Get(res.Header.Get("Location"))
	res.Body.Close()
	cb := res.Header.Get("Location")
	if !strings.Contains(cb, pathGitHubCallback) {
		t.Fatalf("callback: %s", cb)
	}
	v := e.browser("v")
	if r, page := e.getAbs(v, cb); r.StatusCode != http.StatusBadRequest || e.sessionCookie(v) != "" {
		t.Fatalf("another's callback, no state cookie: %d %s", r.StatusCode, page)
	}
	res, _ = noFollow(v).Get(e.srv.URL + pathGitHub + "?next=/")
	res.Body.Close()
	if r, _ := e.getAbs(v, cb); r.StatusCode != http.StatusBadRequest || e.sessionCookie(v) != "" {
		t.Fatalf("another's callback, the victim's own sign-in pending: %d", r.StatusCode)
	}
	if r, page := e.getAbs(v, e.srv.URL+pathGitHubCallback+"?state=x&error=%3Cscript%3E"); r.StatusCode != http.StatusBadRequest || strings.Contains(page, "<script>") {
		t.Fatalf("error reflected with a forged state: %d", r.StatusCode)
	}
}

// Accounts are keyed by GitHub's numeric ID: someone who takes over a
// renamed login gets an account of their own.
func TestAccountsFollowGitHubIDs(t *testing.T) {
	e := newWebEnv(t, nil, nil)
	ctx := t.Context()
	a1, _ := e.db.GitHubAccount(ctx, 1, "ada", "Ada", "ada@example.com", "")
	renamed, _ := e.db.GitHubAccount(ctx, 1, "ada2", "Ada", "ada@example.com", "")
	squat, _ := e.db.GitHubAccount(ctx, 2, "ada", "Ada", "ada@example.com", "")
	if a1.ID != renamed.ID || squat.ID == a1.ID {
		t.Fatalf("accounts: %s %s %s", a1.ID, renamed.ID, squat.ID)
	}
}
