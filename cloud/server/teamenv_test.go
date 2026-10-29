package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/store"
)

// teamEnv is a cloud and its database, for tests that make accounts, teams
// and members through the API.
type teamEnv struct {
	t   *testing.T
	url string
	db  *store.DB
}

// signedInMember is an account with a device signed in.
type signedInMember struct {
	token string
	acc   api.Account
	dev   api.Device
	key   *seal.DeviceKey
}

func newTeamEnv(t *testing.T, cfg Config) *teamEnv {
	srv, db := startWithDB(t, cfg)
	return &teamEnv{t: t, url: srv.URL, db: db}
}

// on is another server, with cfg, over db (e.db: the same database).
func (e *teamEnv) on(cfg Config, db *store.DB) string {
	srv := httptest.NewServer(New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	e.t.Cleanup(srv.Close)
	return srv.URL
}

// signIn makes (or finds) GitHub account gh and signs a new device of it in.
func (e *teamEnv) signIn(gh int64, login, email string) signedInMember {
	e.t.Helper()
	ctx := context.Background()
	acc, err := e.db.GitHubAccount(ctx, gh, login, login, email, "")
	if err != nil {
		e.t.Fatal(err)
	}
	k, _ := seal.NewDeviceKey()
	d, tok, err := e.db.AddDevice(ctx, acc.ID, login+"'s laptop", api.DeviceApp, k.Public())
	if err != nil {
		e.t.Fatal(err)
	}
	return signedInMember{token: tok, acc: acc, dev: d, key: k}
}

// team makes a team owned by owner and gives its ID.
func (e *teamEnv) team(owner signedInMember, name string) string {
	e.t.Helper()
	var team api.Team
	if code, msg := send(e.t, "POST", e.url+api.PathTeams, owner.token, api.TeamParams{Name: &name}, &team); code != http.StatusOK {
		e.t.Fatalf("create team: %d %s", code, msg)
	}
	return team.ID
}

// join has admin invite joiner to team as role, and joiner accept.
func (e *teamEnv) join(admin signedInMember, team, role string, joiner signedInMember) {
	e.t.Helper()
	var inv api.Invite
	if code, msg := send(e.t, "POST", e.url+"/v1/teams/"+team+"/invites", admin.token, api.InviteParams{Role: role}, &inv); code != http.StatusOK {
		e.t.Fatalf("invite: %d %s", code, msg)
	}
	if code, msg := send(e.t, "POST", e.url+api.PathJoin, joiner.token, api.JoinParams{Token: inv.Token}, nil); code != http.StatusOK {
		e.t.Fatalf("join: %d %s", code, msg)
	}
}
