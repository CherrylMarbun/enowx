package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// A team's key goes from device to device, sealed, and the cloud only ever
// carries the sealed copies: an admin's device starts an epoch, a device
// holding it seals it for a new member's, who then reads what the team
// sealed. A member removed loses theirs, and the next epoch's key is
// started without them.
func TestTeamKeysGoFromDeviceToDevice(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{URL: "https://cloud.example.com"})
	type member struct {
		acc, dev, token string
		key             *seal.DeviceKey
	}
	join := func(gh int64, login string) member {
		acc, _ := db.GitHubAccount(ctx, gh, login, "", "", "")
		k, _ := seal.NewDeviceKey()
		d, token, _ := db.AddDevice(ctx, acc.ID, login+"'s laptop", api.DeviceApp, k.Public())
		return member{acc.ID, d.ID, token, k}
	}
	ada, bob, eve := join(1, "ada"), join(2, "bob"), join(3, "eve")
	name := "enowX"
	var team api.Team
	send(t, "POST", srv.URL+api.PathTeams, ada.token, api.TeamParams{Name: &name}, &team)
	tp := srv.URL + "/v1/teams/" + team.ID
	var inv api.Invite
	send(t, "POST", tp+"/invites", ada.token, api.InviteParams{Role: api.RoleMember}, &inv)
	send(t, "POST", srv.URL+api.PathJoin, bob.token, api.JoinParams{Token: inv.Token}, nil)
	share := func(m member, epoch int, keys ...api.DeviceKey) (int, string) {
		return send(t, "PUT", fmt.Sprintf("%s/keys/%d", tp, epoch), m.token, api.ShareKeyParams{Keys: keys}, nil)
	}
	sealFor := func(key []byte, to member) api.DeviceKey {
		s, err := seal.SealKey(key, to.key.Public())
		if err != nil {
			t.Fatal(err)
		}
		return api.DeviceKey{Device: to.dev, Sealed: s}
	}
	held := func(m member) api.TeamKeys {
		var k api.TeamKeys
		send(t, "GET", tp+"/keys", m.token, nil, &k)
		return k
	}

	k1, _ := seal.NewTeamKey()
	if code, _ := share(bob, 1, sealFor(k1, bob)); code != http.StatusForbidden {
		t.Fatalf("a member started an epoch: %d", code)
	}
	if code, msg := share(ada, 1, sealFor(k1, ada)); code != http.StatusNoContent {
		t.Fatalf("ada starts epoch 1: %d %s", code, msg)
	}
	var devices []api.TeamDevice
	send(t, "GET", tp+"/devices", ada.token, nil, &devices)
	missing := []string{}
	for _, d := range devices {
		if !slices.Contains(d.Epochs, 1) {
			missing = append(missing, d.Device.ID)
		}
	}
	if len(devices) != 2 || !slices.Equal(missing, []string{bob.dev}) {
		t.Fatalf("devices: %+v", devices)
	}
	if code, _ := share(ada, 1, sealFor(k1, eve)); code != http.StatusBadRequest {
		t.Fatalf("the key went to eve, not a member: %d", code)
	}
	if code, _ := share(ada, 1, api.DeviceKey{Device: bob.dev, Sealed: []byte("short")}); code != http.StatusBadRequest {
		t.Fatalf("a key of the wrong size: %d", code)
	}
	if code, msg := share(ada, 1, sealFor(k1, bob)); code != http.StatusNoContent {
		t.Fatalf("ada shares with bob: %d %s", code, msg)
	}
	spoiled, _ := seal.NewTeamKey()
	share(bob, 1, sealFor(spoiled, ada)) // bob can't replace ada's copy

	got := held(bob)
	if got.Epoch != 1 || len(got.Keys) != 1 {
		t.Fatalf("bob holds: %+v", got)
	}
	k, err := bob.key.OpenKey(got.Keys[0].Sealed)
	if err != nil || !bytes.Equal(k, k1) {
		t.Fatalf("bob's key: %v", err)
	}
	note, _ := seal.SealItem(k1, "notes/plan.md", 1, []byte("ship it"))
	if text, err := seal.OpenItem(k, "notes/plan.md", 1, note); err != nil || string(text) != "ship it" {
		t.Fatalf("bob reads the team's note: %q %v", text, err)
	}
	if k, _ := ada.key.OpenKey(held(ada).Keys[0].Sealed); !bytes.Equal(k, k1) {
		t.Fatal("bob spoiled ada's copy")
	}

	// Bob goes: his copies go, the epoch moves on, and Ada starts it.
	send(t, "DELETE", tp+"/members/"+bob.acc, ada.token, nil, nil)
	if code, _ := send(t, "GET", tp+"/keys", bob.token, nil, nil); code != http.StatusNotFound {
		t.Fatalf("bob after leaving: %d", code)
	}
	var left int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_keys WHERE device_id = $1`, bob.dev).Scan(&left)
	k2, _ := seal.NewTeamKey()
	if code, _ := share(ada, 2, sealFor(k2, bob)); code != http.StatusBadRequest || left != 0 {
		t.Fatalf("bob after leaving: %d keys, sharing epoch 2 with him: %d", left, code)
	}
	if code, msg := share(ada, 2, sealFor(k2, ada)); code != http.StatusNoContent {
		t.Fatalf("ada starts epoch 2: %d %s", code, msg)
	}
	if got := held(ada); got.Epoch != 2 || len(got.Keys) != 2 {
		t.Fatalf("ada holds: %+v", got)
	}
}
