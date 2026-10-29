package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// The default team's repository's contributors join it as members, by
// their GitHub account's ID; others as viewers. A viewer who contributes
// later becomes a member, unless an admin set their role: theirs stands.
func TestContributorsJoinTheDefaultTeamAsMembers(t *testing.T) {
	var mu sync.Mutex
	contributors := []int64{2}
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/enowx/enowx/contributors" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		var list []map[string]any
		for _, id := range contributors {
			list = append(list, map[string]any{"id": id, "login": "someone"})
		}
		json.NewEncoder(w).Encode(list)
	}))
	t.Cleanup(gh.Close)
	db := storetest.Open(t)
	ctx := t.Context()
	owner, _ := db.GitHubAccount(ctx, 1, "owner", "Owner", "", "")
	team, _ := db.CreateTeam(ctx, owner.ID, "enowX")
	srv := httptest.NewServer(New(Config{URL: "https://cloud.test", GitHubAPIURL: gh.URL, DefaultTeam: team.ID, DefaultTeamRepo: "enowx/enowx"},
		db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)
	keep := contributorsFresh
	contributorsFresh = 0 // read the list every time
	t.Cleanup(func() { contributorsFresh = keep })

	signIn := func(ghID int64, login string) (string, string) {
		acc, _ := db.GitHubAccount(ctx, ghID, login, login, "", "")
		k, _ := seal.NewDeviceKey()
		_, tok, _ := db.AddDevice(ctx, acc.ID, login+"'s Mac", api.DeviceApp, k.Public())
		return acc.ID, tok
	}
	roleIn := func(tok string) string {
		var list []api.Team
		call(t, srv.URL, "GET", api.PathTeams, tok, &list)
		for _, tm := range list {
			if tm.ID == team.ID {
				return tm.Role
			}
		}
		return ""
	}
	_, devTok := signIn(2, "dev")
	_, fanTok := signIn(3, "fan")
	if r := roleIn(devTok); r != api.RoleMember {
		t.Fatalf("a contributor joined as %q", r)
	}
	if r := roleIn(fanTok); r != api.RoleViewer {
		t.Fatalf("someone else joined as %q", r)
	}

	// fan contributes: a member at their next listing.
	mu.Lock()
	contributors = append(contributors, 3)
	mu.Unlock()
	if r := roleIn(fanTok); r != api.RoleMember {
		t.Fatalf("a viewer who contributed is %q", r)
	}

	// An admin's choice stands: made a viewer, a contributor stays one.
	devID, _ := db.GitHubAccount(ctx, 2, "dev", "dev", "", "")
	if err := db.SetRole(ctx, team.ID, owner.ID, devID.ID, api.RoleViewer); err != nil {
		t.Fatal(err)
	}
	if r := roleIn(devTok); r != api.RoleViewer {
		t.Fatalf("a contributor an admin made a viewer is %q", r)
	}
}
