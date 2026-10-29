package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/store"
	"github.com/enowx/enowx/cloud/store/storetest"
)

func start(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	srv, _ := startWithDB(t, cfg)
	return srv
}

func startWithDB(t *testing.T, cfg Config) (*httptest.Server, *store.DB) {
	t.Helper()
	db := storetest.Open(t)
	srv := httptest.NewServer(New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)
	return srv, db
}

// A server says it's up, what it is, and only the sign-in methods it was
// given what they need.
func TestHealthAndInfo(t *testing.T) {
	srv := start(t, Config{URL: "https://cloud.example.com", GitHubClientID: "id", GitHubClientSecret: "secret"})
	res, err := http.Get(srv.URL + api.PathHealth)
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("health: %v %v", res, err)
	}
	res, err = http.Get(srv.URL + api.PathInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var in api.Info
	if err := json.NewDecoder(res.Body).Decode(&in); err != nil {
		t.Fatal(err)
	}
	if in.Name != "enowx-cloud" || in.API != api.Version || in.URL != "https://cloud.example.com" || !slices.Equal(in.SignIn, []string{api.SignInGitHub}) {
		t.Fatalf("info: %+v", in)
	}

	bare := start(t, Config{GitHubClientID: "id"}) // no secret: GitHub can't sign in
	res, err = http.Get(bare.URL + api.PathInfo)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	in = api.Info{}
	json.NewDecoder(res.Body).Decode(&in)
	if in.SignIn == nil || len(in.SignIn) != 0 {
		t.Fatalf("sign-in methods without their setup: %#v", in.SignIn)
	}
	if res, _ := http.Post(srv.URL+api.PathInfo, "application/json", nil); res.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST info: %d", res.StatusCode)
	}
}
