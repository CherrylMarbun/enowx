package client_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
)

// A host signs in with a code another of its account's devices approves,
// and then works with teams as that account.
func TestClientSignsInAndWorksWithTeams(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db := storetest.Open(t)
	srv := httptest.NewServer(server.New(server.Config{}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(srv.Close)

	acc, _ := db.GitHubAccount(ctx, 1, "ada", "Ada", "", "")
	k, _ := seal.NewDeviceKey()
	_, macToken, _ := db.AddDevice(ctx, acc.ID, "Ada's Mac", api.DeviceApp, k.Public())
	mac, err := client.New(srv.URL, macToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.New("ftp://nope", ""); err == nil {
		t.Fatal("an address that isn't http(s) was taken")
	}
	if in, err := mac.Info(ctx); err != nil || in.API != api.Version {
		t.Fatalf("info: %+v %v", in, err)
	}

	host, _ := client.New(srv.URL, "")
	hk, _ := seal.NewDeviceKey()
	dc, err := host.StartSignIn(ctx, api.DeviceCodeParams{Name: "vps", Kind: api.DeviceHost, PublicKey: hk.Public()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.SignInToken(ctx, dc.DeviceCode); !errors.Is(err, client.ErrPending) {
		t.Fatalf("before approval: %v", err)
	}
	dc.Interval = 0 // asks every second, not five
	go func() {
		time.Sleep(300 * time.Millisecond)
		if err := mac.ApproveDevice(ctx, dc.UserCode); err != nil {
			t.Error(err)
		}
	}()
	in, err := host.WaitSignIn(ctx, dc)
	if err != nil || in.Me.Account.ID != acc.ID {
		t.Fatalf("signed in: %+v %v", in, err)
	}
	host.Token = in.Token
	if me, err := host.Me(ctx); err != nil || me.Device.Name != "vps" {
		t.Fatalf("me: %+v %v", me, err)
	}

	team, err := host.CreateTeam(ctx, "enowX")
	if err != nil || team.Role != api.RoleOwner {
		t.Fatalf("create: %+v %v", team, err)
	}
	inv, err := host.Invite(ctx, team.ID, api.InviteParams{Role: api.RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	bob, _ := db.GitHubAccount(ctx, 2, "bob", "", "", "")
	bk, _ := seal.NewDeviceKey()
	_, bobToken, _ := db.AddDevice(ctx, bob.ID, "Bob's", api.DeviceApp, bk.Public())
	bc, _ := client.New(srv.URL, bobToken)
	if joined, err := bc.Join(ctx, inv.Token); err != nil || len(joined.Members) != 2 {
		t.Fatalf("join: %+v %v", joined, err)
	}
	if err := bc.DeleteTeam(ctx, team.ID); !client.IsStatus(err, http.StatusForbidden) {
		t.Fatalf("a member deleted the team: %v", err)
	}
	if err := host.SignOut(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Me(ctx); !client.IsStatus(err, http.StatusUnauthorized) {
		t.Fatalf("signed out: %v", err)
	}
}
