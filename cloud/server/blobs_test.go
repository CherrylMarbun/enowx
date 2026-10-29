package server

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/blobstore"
)

// A team's files: a member puts one, every member gets it back, a viewer
// puts none, one outside the team reaches none, and only its sender or an
// admin deletes it.
func TestTeamBlobs(t *testing.T) {
	store := blobstore.Dir(t.TempDir())
	e := newTeamEnv(t, Config{Blobs: store})
	own := e.signIn(1, "own", "")
	mem := e.signIn(2, "mem", "")
	oth := e.signIn(3, "oth", "")
	vie := e.signIn(4, "vie", "")
	out := e.signIn(5, "out", "")
	team := e.team(own, "t")
	e.join(own, team, api.RoleMember, mem)
	e.join(own, team, api.RoleMember, oth)
	e.join(own, team, api.RoleViewer, vie)
	const id = "0123456789abcdef0123456789abcdef"
	path := e.url + "/v1/teams/" + team + "/blobs/"
	raw := func(method, url, token string, body []byte) (int, []byte) {
		t.Helper()
		req, _ := http.NewRequest(method, url, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data
	}
	sealed := []byte("sealed image bytes")
	cases := []struct {
		name, method, url string
		as                signedInMember
		body              []byte
		want              int
	}{
		{"viewer puts", "PUT", path + id, vie, sealed, 403},
		{"outsider puts", "PUT", path + id, out, sealed, 404},
		{"bad id", "PUT", path + "../../x", mem, sealed, 405}, // cleaned to the team's path
		{"short id", "PUT", path + "abc", mem, sealed, 400},
		{"too large", "PUT", path + id, mem, make([]byte, api.MaxBlob+1), 413},
		{"member puts", "PUT", path + id, mem, sealed, 204},
		{"put again", "PUT", path + id, oth, sealed, 400},
		{"outsider gets", "GET", path + id, out, nil, 404},
		{"another member deletes", "DELETE", path + id, oth, nil, 403},
	}
	for _, c := range cases {
		if code, msg := raw(c.method, c.url, c.as.token, c.body); code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, code, msg, c.want)
		}
	}
	for _, m := range []signedInMember{own, oth, vie} {
		if code, data := raw("GET", path+id, m.token, nil); code != 200 || !bytes.Equal(data, sealed) {
			t.Errorf("%s gets: %d %q", m.acc.GitHub, code, data)
		}
	}
	if code, _ := raw("DELETE", path+id, own.token, nil); code != 204 {
		t.Fatalf("an admin deletes: %d", code)
	}
	if code, _ := raw("GET", path+id, mem.token, nil); code != 404 {
		t.Errorf("a deleted file: %d", code)
	}
	if data, err := store.Get(t.Context(), blobKey(team, id)); err == nil {
		t.Errorf("the store keeps a deleted file: %q", data)
	}
	// A team deleted takes its files along.
	raw("PUT", path+strings.Repeat("a", 32), mem.token, sealed)
	if code, msg := send(t, "DELETE", e.url+"/v1/teams/"+team, own.token, nil, nil); code != 204 {
		t.Fatalf("delete the team: %d %s", code, msg)
	}
	if _, err := store.Get(t.Context(), blobKey(team, strings.Repeat("a", 32))); err == nil {
		t.Error("a deleted team's file stays")
	}
}
