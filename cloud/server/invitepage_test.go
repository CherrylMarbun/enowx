package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// An invite's link opened in a browser by someone with no account: the
// page reads its token, says what it is for and who invites, signs them
// in with GitHub (making their account, which joins the default team as a
// viewer), and joins the team; the link then works no more. Someone
// removed from the default team isn't put back.
func TestInviteJoinsFromTheBrowser(t *testing.T) {
	gh := fakeGitHub(t)
	var handler http.Handler
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(w, r) }))
	t.Cleanup(srv.Close)
	db := storetest.Open(t)
	ctx := t.Context()
	bob, _ := db.GitHubAccount(ctx, 7, "bob", "Bob", "", "")
	home, _ := db.CreateTeam(ctx, bob.ID, "enowX")
	design, _ := db.CreateTeam(ctx, bob.ID, "Design")
	inv, _ := db.CreateInvite(ctx, design.ID, bob.ID, api.InviteParams{Role: api.RoleMember})
	handler = New(Config{URL: srv.URL, GitHubClientID: "cid", GitHubClientSecret: "csecret", GitHubURL: gh.URL, GitHubAPIURL: gh.URL, DefaultTeam: home.ID},
		db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	jar, _ := cookiejar.New(nil)
	browser := &http.Client{Jar: jar}
	read := func(res *http.Response, err error) (*http.Response, string) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, string(b)
	}
	post := func(path string, form url.Values) (*http.Response, string) {
		req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		return read(browser.Do(req))
	}

	// The link: its page runs the one script that reads the token.
	res, page := read(browser.Get(srv.URL + "/invite"))
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src "+inviteScriptHash) || !strings.Contains(page, inviteScript) || !strings.Contains(page, "https://enowx.ai") {
		t.Fatalf("the link's page: %s %s", res.Header.Get("Content-Security-Policy"), page)
	}
	res, page = post("/invite", url.Values{"token": {inv.Token}})
	if res.Request.URL.Path != "/invite" || !strings.Contains(page, "Join Design") || !strings.Contains(page, "Bob invites you to the team, as a member.") || !strings.Contains(page, "Sign in with GitHub") {
		t.Fatalf("the invite, before signing in: %s", page)
	}
	res, page = read(browser.Get(srv.URL + "/v1/auth/github?next=/invite"))
	if res.Request.URL.Path != "/invite" || !strings.Contains(page, `action="/invite/accept"`) || !strings.Contains(page, "@ada") {
		t.Fatalf("the invite, signed in: %s", page)
	}
	if _, page = post("/invite/accept", nil); !strings.Contains(page, "You joined Design as a member.") {
		t.Fatalf("joining: %s", page)
	}
	var adaID string
	db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE github_id = $1`, 42).Scan(&adaID)
	teams, _ := db.Teams(ctx, adaID)
	roles := map[string]string{}
	for _, tm := range teams {
		roles[tm.Name] = tm.Role
	}
	if roles["Design"] != api.RoleMember || roles["enowX"] != api.RoleViewer {
		t.Fatalf("ada's teams: %+v", roles)
	}
	if _, page = post("/invite", url.Values{"token": {inv.Token}}); !strings.Contains(page, "used already") {
		t.Fatalf("the link once used: %s", page)
	}

	// Removed from the default team, ada stays out of it.
	if err := db.RemoveMember(ctx, home.ID, bob.ID, adaID); err != nil {
		t.Fatal(err)
	}
	if joined, err := db.JoinDefaultTeam(ctx, home.ID, adaID, api.RoleViewer); joined || err != nil {
		t.Fatalf("put back in the default team: %v %v", joined, err)
	}
}
