package server

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
)

// An invite planted in a browser by another page (its cookie) neither
// hides the invite whose link is opened next nor gets joined in its place:
// the page always reads the link's token, names the inviter's GitHub login,
// and Join joins only the invite the page showed.
func TestPlantedInviteIsNotJoined(t *testing.T) {
	e := newWebEnv(t, map[string]ghUser{"victim": {ID: 100, Login: "victim", Name: "Vic"}}, nil)
	ctx := t.Context()
	alice, _ := e.db.GitHubAccount(ctx, 1, "alice", "Alice Admin", "", "")
	real, _ := e.db.CreateTeam(ctx, alice.ID, "Acme Engineering")
	realInv, _ := e.db.CreateInvite(ctx, real.ID, alice.ID, api.InviteParams{Role: api.RoleMember})
	// Named like Alice and her team.
	mallory, _ := e.db.GitHubAccount(ctx, 666, "m4llory", "Alice Admin", "", "")
	fake, _ := e.db.CreateTeam(ctx, mallory.ID, "Acme Engineering")
	fakeInv, _ := e.db.CreateInvite(ctx, fake.ID, mallory.ID, api.InviteParams{Role: api.RoleViewer})

	victim := e.browser("victim")
	if res, _ := e.post(victim, "/invite", url.Values{"token": {fakeInv.Token}}, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("planting: %d", res.StatusCode)
	}
	// The real link opened later: its page still reads the link's token.
	if _, page := e.get(victim, "/invite"); !strings.Contains(page, inviteScript) {
		t.Fatalf("an invite kept before hides the link's own token: %s", page)
	}
	_, page := e.post(victim, "/invite", url.Values{"token": {realInv.Token}}, nil)
	if !strings.Contains(page, "@alice") || strings.Contains(page, "m4llory") {
		t.Fatalf("the link's invite isn't the one shown, by its inviter's login: %s", page)
	}
	e.signIn(victim, "/invite")
	_, page = e.get(victim, "/invite")
	shown := regexp.MustCompile(`name="invite" value="([0-9a-f]+)"`).FindStringSubmatch(page)
	if shown == nil {
		t.Fatalf("the invite shown, named for joining: %s", page)
	}

	// Another page plants its invite again before Join.
	e.post(victim, "/invite", url.Values{"token": {fakeInv.Token}}, nil)
	res, page := e.post(victim, "/invite/accept", url.Values{"invite": {shown[1]}}, nil)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("joining with another invite kept than the one shown: %d %s", res.StatusCode, page)
	}
	if _, page := e.get(victim, "/invite"); strings.Contains(page, "Join Acme") {
		t.Errorf("the planted invite's cookie stays after a refused join: %s", page)
	}
	var vicID string
	e.db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE github_id = 100`).Scan(&vicID)
	if teams, _ := e.db.Teams(ctx, vicID); len(teams) != 0 {
		t.Fatalf("joined %+v", teams)
	}

	// The real link, opened again, joins the real team.
	_, page = e.post(victim, "/invite", url.Values{"token": {realInv.Token}}, nil)
	shown = regexp.MustCompile(`name="invite" value="([0-9a-f]+)"`).FindStringSubmatch(page)
	if shown == nil {
		t.Fatalf("the real invite again: %s", page)
	}
	if res, page := e.post(victim, "/invite/accept", url.Values{"invite": {shown[1]}}, nil); res.StatusCode != http.StatusOK {
		t.Fatalf("joining the invite shown: %d %s", res.StatusCode, page)
	}
	teams, _ := e.db.Teams(ctx, vicID)
	if len(teams) != 1 || teams[0].ID != real.ID {
		t.Fatalf("teams: %+v", teams)
	}
}
