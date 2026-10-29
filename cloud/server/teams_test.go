package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
)

// A team from its making to its members going: invites work once, roles
// limit who does what, the owner hands the team over before leaving, and a
// member removed loses the team's keys and sees the team no more.
func TestTeamsInvitesAndRoles(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{URL: "https://cloud.example.com"})
	signedIn := func(gh int64, login string) (string, string) {
		acc, _ := db.GitHubAccount(ctx, gh, login, "", "", "")
		k, _ := seal.NewDeviceKey()
		_, token, err := db.AddDevice(ctx, acc.ID, login+"'s Mac", api.DeviceApp, k.Public())
		if err != nil {
			t.Fatal(err)
		}
		return acc.ID, token
	}
	ada, adaT := signedIn(1, "ada")
	bob, bobT := signedIn(2, "bob")
	eve, eveT := signedIn(3, "eve")
	u := func(path string) string { return srv.URL + path }

	name := "enowX"
	var team api.Team
	if code, msg := send(t, "POST", u(api.PathTeams), adaT, api.TeamParams{Name: &name}, &team); code != http.StatusOK || team.Role != api.RoleOwner {
		t.Fatalf("create: %d %s %+v", code, msg, team)
	}
	tp := "/v1/teams/" + team.ID
	if code, _ := send(t, "GET", u(tp), eveT, nil, nil); code != http.StatusNotFound {
		t.Fatalf("eve sees a team she isn't in: %d", code)
	}

	// Ada invites Bob as a member; the link's token joins him, once.
	var inv api.Invite
	if code, msg := send(t, "POST", u(tp+"/invites"), adaT, api.InviteParams{Role: api.RoleMember}, &inv); code != http.StatusOK || !strings.HasPrefix(inv.URL, "https://cloud.example.com/invite#enxi_") {
		t.Fatalf("invite: %d %s %+v", code, msg, inv)
	}
	if code, _ := send(t, "POST", u(tp+"/invites"), adaT, api.InviteParams{Role: api.RoleOwner}, nil); code != http.StatusBadRequest {
		t.Fatalf("an invite as owner: %d", code)
	}
	var joined api.Team
	if code, msg := send(t, "POST", u(api.PathJoin), bobT, api.JoinParams{Token: inv.URL}, &joined); code != http.StatusOK || joined.Role != api.RoleMember || len(joined.Members) != 2 {
		t.Fatalf("bob joins: %d %s %+v", code, msg, joined)
	}
	if code, _ := send(t, "POST", u(api.PathJoin), eveT, api.JoinParams{Token: inv.Token}, nil); code != http.StatusNotFound {
		t.Fatalf("the invite worked twice: %d", code)
	}

	// A member can't invite, rename, or change roles.
	if code, _ := send(t, "POST", u(tp+"/invites"), bobT, api.InviteParams{Role: api.RoleViewer}, nil); code != http.StatusForbidden {
		t.Fatalf("bob invites: %d", code)
	}
	renamed := "bob's"
	if code, _ := send(t, "PATCH", u(tp), bobT, api.TeamParams{Name: &renamed}, nil); code != http.StatusForbidden {
		t.Fatalf("bob renames: %d", code)
	}
	if code, _ := send(t, "PATCH", u(tp+"/members/"+bob), bobT, api.RoleParams{Role: api.RoleAdmin}, nil); code != http.StatusForbidden {
		t.Fatalf("bob makes himself admin: %d", code)
	}

	// Invites are listed for admins, without their tokens, and withdrawn.
	send(t, "POST", u(tp+"/invites"), adaT, api.InviteParams{Role: api.RoleViewer, Email: "eve@example.com"}, &inv)
	var open []api.Invite
	send(t, "GET", u(tp+"/invites"), adaT, nil, &open)
	if len(open) != 1 || open[0].Token != "" || open[0].Email != "eve@example.com" {
		t.Fatalf("open invites: %+v", open)
	}
	if code, _ := send(t, "DELETE", u(tp+"/invites/"+open[0].ID), adaT, nil, nil); code != http.StatusNoContent {
		t.Fatalf("withdraw: %d", code)
	}
	if code, _ := send(t, "POST", u(api.PathJoin), eveT, api.JoinParams{Token: inv.Token}, nil); code != http.StatusNotFound {
		t.Fatalf("a withdrawn invite worked: %d", code)
	}

	// The owner can't leave; handing the team to Bob makes Ada an admin.
	if code, _ := send(t, "DELETE", u(tp+"/members/"+ada), adaT, nil, nil); code != http.StatusForbidden {
		t.Fatalf("the owner left: %d", code)
	}
	if code, msg := send(t, "PATCH", u(tp+"/members/"+bob), adaT, api.RoleParams{Role: api.RoleOwner}, nil); code != http.StatusNoContent {
		t.Fatalf("hand over: %d %s", code, msg)
	}
	send(t, "GET", u(tp), adaT, nil, &team)
	if team.Role != api.RoleAdmin {
		t.Fatalf("ada after handing over: %s", team.Role)
	}

	// Bob, owner now, removes Ada: her sealed keys go, the epoch moves on,
	// and the team is gone from her list.
	db.ExecContext(ctx, `INSERT INTO team_keys (team_id, epoch, device_id, sealed) SELECT ?, 1, id, x'00' FROM devices WHERE account_id = ?`, team.ID, ada)
	if code, msg := send(t, "DELETE", u(tp+"/members/"+ada), bobT, nil, nil); code != http.StatusNoContent {
		t.Fatalf("remove ada: %d %s", code, msg)
	}
	var keys int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_keys WHERE team_id = ?`, team.ID).Scan(&keys)
	var mine []api.Team
	send(t, "GET", u(api.PathTeams), adaT, nil, &mine)
	send(t, "GET", u(tp), bobT, nil, &team)
	if keys != 0 || len(mine) != 0 || team.Epoch != 2 || len(team.Members) != 1 {
		t.Fatalf("after removing ada: keys %d, her teams %d, epoch %d, members %d", keys, len(mine), team.Epoch, len(team.Members))
	}
	_ = eve
	if code, _ := send(t, "DELETE", u(tp), bobT, nil, nil); code != http.StatusNoContent {
		t.Fatalf("delete: %d", code)
	}
}
