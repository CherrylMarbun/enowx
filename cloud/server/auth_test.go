package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// call asks srv with token, decoding the answer into out when given.
func call(t *testing.T, base, method, path, token string, out any) int {
	t.Helper()
	req, _ := http.NewRequest(method, base+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil && res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return res.StatusCode
}

// A device's token says who asks; signed out, removed, or never given, it
// opens nothing. The database keeps no token, and one account can't touch
// another's devices.
func TestDevicesSignInAndOut(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{})
	ada, err := db.GitHubAccount(ctx, 101, "ada", "Ada Lovelace", "ada@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	key := func() []byte { k, _ := seal.NewDeviceKey(); return k.Public() }
	mac, macToken, err := db.AddDevice(ctx, ada.ID, "Ada's Mac", api.DeviceApp, key())
	if err != nil {
		t.Fatal(err)
	}
	vps, vpsToken, _ := db.AddDevice(ctx, ada.ID, "vps", api.DeviceHost, key())
	bob, _ := db.GitHubAccount(ctx, 202, "bob", "", "", "")
	bobDev, bobToken, _ := db.AddDevice(ctx, bob.ID, "Bob's laptop", api.DeviceApp, key())

	if code := call(t, srv.URL, "GET", api.PathMe, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", code)
	}
	if code := call(t, srv.URL, "GET", api.PathMe, "enxc_madeup", nil); code != http.StatusUnauthorized {
		t.Fatalf("a token never given: %d", code)
	}
	var me api.Me
	if code := call(t, srv.URL, "GET", api.PathMe, macToken, &me); code != http.StatusOK || me.Account.ID != ada.ID || me.Device.ID != mac.ID || !me.Device.Current {
		t.Fatalf("me: %d %+v", code, me)
	}
	var list []api.Device
	call(t, srv.URL, "GET", api.PathDevices, macToken, &list)
	if len(list) != 2 {
		t.Fatalf("ada's devices: %+v", list)
	}
	for _, d := range list {
		if d.Current != (d.ID == mac.ID) {
			t.Errorf("%s current %v", d.Name, d.Current)
		}
	}
	var leaked int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tokens WHERE hash = ? OR CAST(hash AS TEXT) LIKE '%enxc_%'`, macToken).Scan(&leaked)
	if leaked != 0 || !strings.HasPrefix(macToken, "enxc_") {
		t.Fatal("a token is kept as it is")
	}

	if code := call(t, srv.URL, "DELETE", "/v1/devices/"+bobDev.ID, macToken, nil); code != http.StatusNotFound {
		t.Fatalf("ada removed bob's device: %d", code)
	}
	if code := call(t, srv.URL, "GET", api.PathMe, bobToken, nil); code != http.StatusOK {
		t.Fatalf("bob after ada's try: %d", code)
	}
	if code := call(t, srv.URL, "DELETE", "/v1/devices/"+vps.ID, macToken, nil); code != http.StatusNoContent {
		t.Fatalf("remove the vps: %d", code)
	}
	if code := call(t, srv.URL, "GET", api.PathMe, vpsToken, nil); code != http.StatusUnauthorized {
		t.Fatalf("the vps removed still signed in: %d", code)
	}
	if code := call(t, srv.URL, "POST", api.PathSignOut, macToken, nil); code != http.StatusNoContent {
		t.Fatalf("sign out: %d", code)
	}
	if code := call(t, srv.URL, "GET", api.PathMe, macToken, nil); code != http.StatusUnauthorized {
		t.Fatalf("signed out still signed in: %d", code)
	}

	// Signing in again with GitHub finds the same account, renamed.
	again, err := db.GitHubAccount(ctx, 101, "ada-l", "Ada", "", "")
	if err != nil || again.ID != ada.ID || again.GitHub != "ada-l" {
		t.Fatalf("again: %+v %v", again, err)
	}
	var actions int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit WHERE account_id = ?`, ada.ID).Scan(&actions)
	if actions < 5 { // created, 2 added, 2 removed
		t.Fatalf("audit has %d of ada's actions", actions)
	}
}
