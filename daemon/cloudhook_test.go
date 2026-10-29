package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// hookedCloud is a real cloud server whose requests can go through a
// hook, standing in for a malicious or curious server operator.
type hookedCloud struct {
	db   *store.DB
	url  string
	mu   sync.Mutex
	hook func(w http.ResponseWriter, r *http.Request, next http.Handler)
}

func newHookedCloud(t *testing.T) *hookedCloud {
	t.Helper()
	sc := &hookedCloud{db: storetest.Open(t)}
	real := server.New(server.Config{URL: "http://cloud.test"}, sc.db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sc.mu.Lock()
		h := sc.hook
		sc.mu.Unlock()
		if h != nil {
			h(w, r, real)
			return
		}
		real.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	sc.url = ts.URL
	return sc
}

func (sc *hookedCloud) setHook(h func(w http.ResponseWriter, r *http.Request, next http.Handler)) {
	sc.mu.Lock()
	sc.hook = h
	sc.mu.Unlock()
}

// teamOfHosts signs a host in for each of names, the first making team
// Acme as its owner, the others joining it with roles[i].
func teamOfHosts(t *testing.T, sc *hookedCloud, names []string, roles []string) (string, []teamHost) {
	t.Helper()
	ctx := context.Background()
	var hosts []teamHost
	var teamID string
	for i, login := range names {
		h := signInHost(t, sc.db, sc.url, int64(100+i), login)
		if i == 0 {
			team, err := sc.db.CreateTeam(ctx, h.acc, "Acme")
			if err != nil {
				t.Fatal(err)
			}
			teamID = team.ID
			h.srv.learnTeams(h.acc, []api.Team{{ID: teamID, Name: "Acme", Role: api.RoleOwner}})
		} else {
			joinTeam(t, sc, teamID, hosts[0], h, roles[i])
		}
		hosts = append(hosts, h)
	}
	return teamID, hosts
}

func joinTeam(t *testing.T, sc *hookedCloud, team string, owner, h teamHost, role string) {
	t.Helper()
	ctx := context.Background()
	inv, err := sc.db.CreateInvite(ctx, team, owner.acc, api.InviteParams{Role: role})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sc.db.AcceptInvite(ctx, inv.Token, h.acc); err != nil {
		t.Fatal(err)
	}
	h.srv.learnTeams(h.acc, []api.Team{{ID: team, Name: "Acme", Role: role}})
}

// cloudAs is a cloud client with host h's own token: what anyone holding
// that token (a stolen token, a malicious member's own) can do directly.
func cloudAs(t *testing.T, sc *hookedCloud, h teamHost) *cloudclient.Client {
	t.Helper()
	st, err := h.srv.cloud.store.Get()
	if err != nil {
		t.Fatal(err)
	}
	c, err := cloudclient.New(sc.url, st.Token)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func deviceOf(t *testing.T, h teamHost) string {
	t.Helper()
	st, err := h.srv.cloud.store.Get()
	if err != nil {
		t.Fatal(err)
	}
	return st.DeviceID
}

// writeInSpace writes a file into h's copy of team's space, as a member
// with a modified daemon (or by hand) would, bypassing the RPC checks.
func writeInSpace(t *testing.T, h teamHost, team, rel string, data []byte) {
	t.Helper()
	full := filepath.Join(h.srv.teamsDir(), team, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
