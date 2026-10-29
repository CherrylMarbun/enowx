package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// A member who lost every device gets the team's key back on a new one
// with their recovery key: the team's key was sealed to it like to their
// devices; nobody else's device recovers through it.
func TestRecoveryKeyBringsTheTeamsKeyBack(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{URL: "https://cloud.example.com"})
	acc, _ := db.GitHubAccount(ctx, 1, "ada", "ada", "", "")
	old, _ := seal.NewDeviceKey()
	oldDev, oldTok, _ := db.AddDevice(ctx, acc.ID, "ada's laptop", api.DeviceApp, old.Public())
	rec, _ := seal.NewDeviceKey()
	var recDev api.Device
	// The token alone sets none: the device opens a challenge sealed to it.
	if code, _ := send(t, "POST", srv.URL+api.PathRecovery, oldTok, api.RecoveryParams{PublicKey: rec.Public()}, nil); code != http.StatusForbidden {
		t.Fatalf("a recovery key set with no proof: %d", code)
	}
	var ch api.RecoveryChallenge
	call(t, srv.URL, "GET", api.PathRecoveryChallenge, oldTok, &ch)
	proof, err := old.OpenKey(ch.Sealed)
	if err != nil {
		t.Fatal(err)
	}
	if code, msg := send(t, "POST", srv.URL+api.PathRecovery, oldTok, api.RecoveryParams{PublicKey: rec.Public(), Proof: proof}, &recDev); code != http.StatusOK || !recDev.Recovery {
		t.Fatalf("recovery key set: %d %s %+v", code, msg, recDev)
	}
	name := "Design"
	var team api.Team
	send(t, "POST", srv.URL+api.PathTeams, oldTok, api.TeamParams{Name: &name}, &team)
	k1, _ := seal.NewTeamKey()
	toOld, _ := seal.SealKey(k1, old.Public())
	toRec, _ := seal.SealKey(k1, rec.Public())
	if code, msg := send(t, "PUT", fmt.Sprintf("%s/v1/teams/%s/keys/1", srv.URL, team.ID), oldTok,
		api.ShareKeyParams{Keys: []api.DeviceKey{{Device: oldDev.ID, Sealed: toOld}, {Device: recDev.ID, Sealed: toRec}}}, nil); code != http.StatusNoContent {
		t.Fatalf("the key shared: %d %s", code, msg)
	}

	// Every device lost: a new one, and the recovery key typed in.
	db.RemoveDevice(ctx, acc.ID, oldDev.ID)
	fresh, _ := seal.NewDeviceKey()
	_, freshTok, _ := db.AddDevice(ctx, acc.ID, "ada's new laptop", api.DeviceApp, fresh.Public())
	var held api.TeamKeys
	if code, msg := send(t, "GET", srv.URL+"/v1/teams/"+team.ID+"/recovery", freshTok, nil, &held); code != http.StatusOK || len(held.Keys) != 1 {
		t.Fatalf("the recovery key's copies: %d %s %+v", code, msg, held)
	}
	key, err := rec.OpenKey(held.Keys[0].Sealed)
	if err != nil || !bytes.Equal(key, k1) {
		t.Fatalf("opened: %v", err)
	}
	mine, _ := seal.SealKey(key, fresh.Public())
	if code, msg := send(t, "POST", srv.URL+"/v1/teams/"+team.ID+"/recovery", freshTok, api.RecoverParams{Epoch: 1, Sealed: mine}, nil); code != http.StatusNoContent {
		t.Fatalf("recovered: %d %s", code, msg)
	}
	var now api.TeamKeys
	send(t, "GET", srv.URL+"/v1/teams/"+team.ID+"/keys", freshTok, nil, &now)
	if len(now.Keys) != 1 {
		t.Fatalf("the new device's keys: %+v", now)
	}
	if got, err := fresh.OpenKey(now.Keys[0].Sealed); err != nil || !bytes.Equal(got, k1) {
		t.Fatalf("the new device's key: %v", err)
	}

	// Another account's device gets nothing through ada's recovery key.
	bob, _ := db.GitHubAccount(ctx, 2, "bob", "bob", "", "")
	bk, _ := seal.NewDeviceKey()
	_, bobTok, _ := db.AddDevice(ctx, bob.ID, "bob's", api.DeviceApp, bk.Public())
	if code, _ := send(t, "GET", srv.URL+"/v1/teams/"+team.ID+"/recovery", bobTok, nil, nil); code != http.StatusNotFound {
		t.Fatalf("one not a member read the recovery copies: %d", code)
	}
}
