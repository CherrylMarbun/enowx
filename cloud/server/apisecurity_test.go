package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/store"
)

// A device that wrote team items is signed out like any other: removed
// from another device of the account, or signing itself out, its token
// opens nothing more (a lost laptop doesn't stay signed in).
func TestDeviceThatWroteItemsSignsOut(t *testing.T) {
	e := newTeamEnv(t, Config{})
	laptop := e.signIn(1, "ada", "")
	phone := e.signIn(1, "ada", "")
	desk := e.signIn(1, "ada", "")
	team := e.team(laptop, "t")
	put := func(m signedInMember, id string) {
		t.Helper()
		if code, msg := send(t, "PUT", e.url+"/v1/teams/"+team+"/items/"+id, m.token,
			api.PutItemParams{Kind: api.ItemNote, Version: 1, Epoch: 1, Sealed: []byte("x")}, nil); code != http.StatusOK {
			t.Fatalf("put: %d %s", code, msg)
		}
	}
	put(laptop, "0123456789abcdef0123456789abcdef")
	put(desk, "0123456789abcdef0123456789abcdee")

	if code, msg := send(t, "DELETE", e.url+"/v1/devices/"+laptop.dev.ID, phone.token, nil, nil); code != http.StatusNoContent {
		t.Errorf("removing a device that wrote an item: %d %q", code, msg)
	}
	if code := call(t, e.url, "GET", api.PathMe, laptop.token, nil); code != http.StatusUnauthorized {
		t.Errorf("the removed device's token: %d", code)
	}
	if code, msg := send(t, "POST", e.url+api.PathSignOut, desk.token, nil, nil); code != http.StatusNoContent {
		t.Errorf("signing out a device that wrote an item: %d %q", code, msg)
	}
	if code := call(t, e.url, "GET", api.PathMe, desk.token, nil); code != http.StatusUnauthorized {
		t.Errorf("the signed-out device's token: %d", code)
	}
	var items api.Items
	if code := call(t, e.url, "GET", "/v1/teams/"+team+"/items", phone.token, &items); code != http.StatusOK || len(items.Items) != 2 {
		t.Errorf("the items the devices wrote: %d %+v", code, items)
	}
}

// Members' emails are their own and the admins' to see, and other
// accounts' devices show non-admins only their keys: with a default team,
// anyone signing in to the server would otherwise read everyone's.
func TestMembersDontSeeOthersEmailsOrDevices(t *testing.T) {
	e := newTeamEnv(t, Config{})
	owner := e.signIn(1, "owner", "owner@corp.example")
	team := e.team(owner, "Everyone")
	srv := e.on(Config{DefaultTeam: team}, e.db)
	bob := e.signIn(2, "bob", "bob@example.com")
	eve := e.signIn(3, "eve", "eve@example.com")
	call(t, srv, "GET", api.PathTeams, bob.token, nil) // joins as a viewer
	call(t, srv, "GET", api.PathTeams, eve.token, nil)
	send(t, "PATCH", srv+"/v1/teams/"+team+"/members/"+bob.acc.ID, owner.token, api.RoleParams{Role: api.RoleMember}, nil)

	for _, m := range []signedInMember{bob, eve} {
		var tm api.Team
		if code := call(t, srv, "GET", "/v1/teams/"+team, m.token, &tm); code != http.StatusOK || len(tm.Members) != 3 {
			t.Fatalf("team: %d %+v", code, tm)
		}
		for _, mm := range tm.Members {
			if mine := mm.Account.ID == m.acc.ID; (mm.Account.Email != "") != mine {
				t.Errorf("%s sees %s's email %q", m.acc.GitHub, mm.Account.GitHub, mm.Account.Email)
			}
		}
		var devs []api.TeamDevice
		if code := call(t, srv, "GET", "/v1/teams/"+team+"/devices", m.token, &devs); code != http.StatusOK || len(devs) != 3 {
			t.Fatalf("devices: %d %+v", code, devs)
		}
		for _, d := range devs {
			mine := d.Account == m.acc.ID
			if mine && d.Device.Name == "" {
				t.Errorf("%s doesn't see their own device's name", m.acc.GitHub)
			}
			if !mine && (d.Device.Name != "" || !d.Device.LastSeen.Equal(d.Device.Created) || len(d.Device.PublicKey) != 32) {
				t.Errorf("%s sees another's device: %+v", m.acc.GitHub, d.Device)
			}
		}
	}

	var tm api.Team
	call(t, srv, "GET", "/v1/teams/"+team, owner.token, &tm)
	var devs []api.TeamDevice
	call(t, srv, "GET", "/v1/teams/"+team+"/devices", owner.token, &devs)
	for _, mm := range tm.Members {
		if mm.Account.Email == "" {
			t.Errorf("the owner doesn't see %s's email", mm.Account.GitHub)
		}
	}
	for _, d := range devs {
		if d.Device.Name == "" {
			t.Errorf("the owner doesn't see a device's name: %+v", d)
		}
	}
}

// A viewer seals the team's key for their own devices only. A copy a
// member sealed with another key is told apart by the epoch's check and
// dropped by its device, for the real key to be sealed to it again.
func TestKeyCopiesCantBePlanted(t *testing.T) {
	e := newTeamEnv(t, Config{})
	ada := e.signIn(1, "ada", "")
	bob := e.signIn(2, "bob", "")
	vic := e.signIn(3, "vic", "")
	team := e.team(ada, "t")
	e.join(ada, team, api.RoleMember, bob)
	e.join(ada, team, api.RoleViewer, vic)
	keys := func(m signedInMember) api.TeamKeys {
		t.Helper()
		var k api.TeamKeys
		if code := call(t, e.url, "GET", "/v1/teams/"+team+"/keys", m.token, &k); code != http.StatusOK {
			t.Fatalf("keys: %d", code)
		}
		return k
	}
	share := func(from signedInMember, check []byte, to ...signedInMember) (int, string) {
		t.Helper()
		var dk []api.DeviceKey
		for _, m := range to {
			sealed := make([]byte, api.SealedKeySize) // no key: bytes that open to nothing
			if m.key != nil {
				sealed, _ = seal.SealKey(realKey, m.key.Public())
			}
			dk = append(dk, api.DeviceKey{Device: m.dev.ID, Sealed: sealed})
		}
		return send(t, "PUT", e.url+"/v1/teams/"+team+"/keys/1", from.token, api.ShareKeyParams{Keys: dk, Check: check}, nil)
	}

	if code, msg := share(ada, seal.KeyCheck(realKey), ada, bob, vic); code != http.StatusNoContent {
		t.Fatalf("start epoch: %d %s", code, msg)
	}
	got := keys(bob)
	if len(got.Checks) != 1 || got.Checks[0].Epoch != 1 || !bytes.Equal(got.Checks[0].Check, seal.KeyCheck(realKey)) {
		t.Fatalf("the epoch's check: %+v", got.Checks)
	}

	adaNew := e.signIn(1, "ada", "")
	vicNew := e.signIn(3, "vic", "")
	if code, _ := share(vic, nil, adaNew); code != http.StatusForbidden {
		t.Errorf("a viewer sealed for another account's device: %d", code)
	}
	if code, msg := share(vic, nil, vicNew); code != http.StatusNoContent {
		t.Errorf("a viewer sealing for their own new device: %d %s", code, msg)
	}

	// Bob, a member, seals something else to ada's new device first.
	if code, msg := share(bob, nil, signedInMember{dev: adaNew.dev}); code != http.StatusNoContent {
		t.Fatalf("bob's copy: %d %s", code, msg)
	}
	got = keys(adaNew)
	if len(got.Keys) != 1 {
		t.Fatalf("ada's new device's copies: %+v", got)
	}
	if k, err := adaNew.key.OpenKey(got.Keys[0].Sealed); err == nil && seal.CheckKey(k, got.Checks[0].Check) {
		t.Fatal("the planted copy passes the check")
	}
	if code, msg := send(t, "DELETE", e.url+"/v1/teams/"+team+"/keys/1", adaNew.token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("dropping its own copy: %d %s", code, msg)
	}
	if code, msg := share(ada, nil, adaNew); code != http.StatusNoContent {
		t.Fatalf("the real copy: %d %s", code, msg)
	}
	got = keys(adaNew)
	if len(got.Keys) != 1 {
		t.Fatalf("ada's new device's copies: %+v", got)
	}
	if k, err := adaNew.key.OpenKey(got.Keys[0].Sealed); err != nil || !seal.CheckKey(k, got.Checks[0].Check) {
		t.Fatalf("the real copy: %v", err)
	}
	if len(keys(bob).Keys) != 1 || len(keys(ada).Keys) != 1 {
		t.Error("a device's drop took others' copies")
	}
}

// realKey is the team key TestKeyCopiesCantBePlanted shares.
var realKey = bytes.Repeat([]byte{7}, seal.KeySize)

// A recovery challenge proves a device's key once, and only for the
// device it was sealed to.
func TestRecoveryChallengeProvesOnce(t *testing.T) {
	e := newTeamEnv(t, Config{})
	laptop := e.signIn(1, "ada", "")
	phone := e.signIn(1, "ada", "")
	rec, _ := seal.NewDeviceKey()
	challenge := func(m signedInMember) []byte {
		t.Helper()
		var ch api.RecoveryChallenge
		if code := call(t, e.url, "GET", api.PathRecoveryChallenge, m.token, &ch); code != http.StatusOK {
			t.Fatalf("challenge: %d", code)
		}
		proof, err := m.key.OpenKey(ch.Sealed)
		if err != nil {
			t.Fatalf("the challenge isn't sealed to the device: %v", err)
		}
		return proof
	}
	set := func(m signedInMember, proof []byte) int {
		t.Helper()
		code, _ := send(t, "POST", e.url+api.PathRecovery, m.token, api.RecoveryParams{PublicKey: rec.Public(), Proof: proof}, nil)
		return code
	}
	proof := challenge(laptop)
	if code := set(phone, proof); code != http.StatusForbidden {
		t.Errorf("another device's proof: %d", code)
	}
	if code := set(laptop, proof); code != http.StatusOK {
		t.Errorf("the proof: %d", code)
	}
	if code := set(laptop, proof); code != http.StatusForbidden {
		t.Errorf("the proof again: %d", code)
	}
	challenge(phone)
	if code := set(phone, make([]byte, seal.KeySize)); code != http.StatusForbidden {
		t.Errorf("a wrong proof: %d", code)
	}
}

// Names the cloud keeps and shows lose control and direction characters
// and are capped, on a character's boundary.
func TestNamesAreCleanedAndCapped(t *testing.T) {
	e := newTeamEnv(t, Config{})
	ada := e.signIn(1, "ada", "")
	long := "Acme\u202e\x1b[31m\u200b " + strings.Repeat("é", 200)
	var tm api.Team
	if code, msg := send(t, "POST", e.url+api.PathTeams, ada.token, api.TeamParams{Name: &long}, &tm); code != http.StatusOK {
		t.Fatalf("create: %d %s", code, msg)
	}
	clean := func(what, s string, max int) {
		t.Helper()
		if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || strings.ContainsAny(s, "\u202e\x1b\u200b") {
			t.Errorf("%s kept: %q", what, s)
		}
	}
	clean("team name", tm.Name, 64)
	send(t, "PATCH", e.url+"/v1/teams/"+tm.ID, ada.token, api.TeamParams{Name: &long}, nil)
	call(t, e.url, "GET", "/v1/teams/"+tm.ID, ada.token, &tm)
	clean("renamed team", tm.Name, 64)

	d, _, err := e.db.AddDevice(context.Background(), ada.acc.ID, long, api.DeviceApp, ada.key.Public())
	if err != nil {
		t.Fatal(err)
	}
	clean("device name", d.Name, 64)

	send(t, "POST", e.url+"/v1/teams/"+tm.ID+"/invites", ada.token, api.InviteParams{Role: api.RoleViewer, Email: strings.Repeat("a", 300) + "@example.com"}, nil)
	var invs []api.Invite
	call(t, e.url, "GET", "/v1/teams/"+tm.ID+"/invites", ada.token, &invs)
	if len(invs) != 1 || len(invs[0].Email) > 254 {
		t.Errorf("invites: %+v", invs)
	}

	// A detail ending mid-character at 500 bytes is cut whole.
	if code, msg := send(t, "POST", e.url+"/v1/teams/"+tm.ID+"/audit", ada.token,
		api.AuditParams{Action: api.AuditRulesChanged, Detail: strings.Repeat("a", 499) + strings.Repeat("é", 100)}, nil); code != http.StatusNoContent {
		t.Fatalf("audit: %d %s", code, msg)
	}
	var log []api.AuditEntry
	call(t, e.url, "GET", "/v1/teams/"+tm.ID+"/audit", ada.token, &log)
	if len(log) == 0 || log[0].Action != api.AuditRulesChanged {
		t.Fatalf("audit: %+v", log)
	}
	clean("audit detail", log[0].Detail, 500)
}

// The database's errors go to the log, not to who asked.
func TestServerErrorsStayInTheLog(t *testing.T) {
	e := newTeamEnv(t, Config{})
	ada := e.signIn(1, "ada", "")
	team := e.team(ada, "t")
	if _, err := e.db.ExecContext(context.Background(), `DROP TABLE audit`); err != nil {
		t.Fatal(err)
	}
	if code, msg := send(t, "GET", e.url+"/v1/teams/"+team+"/audit", ada.token, nil, nil); code != http.StatusInternalServerError || msg != "something went wrong on the server" {
		t.Errorf("a failing query: %d %q", code, msg)
	}

	down, err := store.Open(context.Background(), store.Config{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	down.Close()
	res, err := http.Get(e.on(Config{}, down) + api.PathHealth)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body api.Error
	json.NewDecoder(res.Body).Decode(&body)
	if res.StatusCode != http.StatusServiceUnavailable || strings.Contains(body.Error, "sql") || strings.Contains(body.Error, "closed") {
		t.Errorf("health with the database down: %d %q", res.StatusCode, body.Error)
	}
}
