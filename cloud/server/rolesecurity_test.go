package server

import (
	"net/http"
	"testing"

	"github.com/enowx/enowx/cloud/api"
)

// Each role does what it may and no more, and a team is as missing to one
// outside it as a team that doesn't exist.
func TestRolesLimitEachMember(t *testing.T) {
	e := newTeamEnv(t, Config{})
	own := e.signIn(1, "own", "")
	adm := e.signIn(2, "adm", "")
	mem := e.signIn(3, "mem", "")
	vie := e.signIn(4, "vie", "")
	out := e.signIn(5, "out", "")
	team := e.team(own, "t")
	e.join(own, team, api.RoleAdmin, adm)
	e.join(own, team, api.RoleMember, mem)
	e.join(own, team, api.RoleViewer, vie)
	tp := e.url + "/v1/teams/" + team
	op := e.url + "/v1/teams/" + e.team(out, "outsider's")
	sealed := make([]byte, api.SealedKeySize)
	name := "x"
	cases := []struct {
		name, method, url string
		as                signedInMember
		body              any
		want              int
	}{
		{"admin makes owner", "PATCH", tp + "/members/" + mem.acc.ID, adm, api.RoleParams{Role: api.RoleOwner}, 403},
		{"admin demotes owner", "PATCH", tp + "/members/" + own.acc.ID, adm, api.RoleParams{Role: api.RoleViewer}, 403},
		{"admin removes owner", "DELETE", tp + "/members/" + own.acc.ID, adm, nil, 403},
		{"owner leaves", "DELETE", tp + "/members/" + own.acc.ID, own, nil, 403},
		{"member promotes self", "PATCH", tp + "/members/" + mem.acc.ID, mem, api.RoleParams{Role: api.RoleAdmin}, 403},
		{"member removes admin", "DELETE", tp + "/members/" + adm.acc.ID, mem, nil, 403},
		{"member invites", "POST", tp + "/invites", mem, api.InviteParams{Role: api.RoleViewer}, 403},
		{"admin invites owner", "POST", tp + "/invites", adm, api.InviteParams{Role: api.RoleOwner}, 400},
		{"member lists invites", "GET", tp + "/invites", mem, nil, 403},
		{"member renames", "PATCH", tp, mem, api.TeamParams{Name: &name}, 403},
		{"admin deletes team", "DELETE", tp, adm, nil, 403},
		{"member reads audit", "GET", tp + "/audit", mem, nil, 403},
		{"viewer records audit", "POST", tp + "/audit", vie, api.AuditParams{Action: api.AuditRead}, 403},
		{"member records cloud action", "POST", tp + "/audit", mem, api.AuditParams{Action: "member.removed", Detail: mem.acc.ID}, 400},
		{"viewer puts item", "PUT", tp + "/items/0123456789abcdef", vie, api.PutItemParams{Kind: api.ItemNote, Version: 1, Epoch: 1, Sealed: []byte("x")}, 403},
		{"member starts epoch", "PUT", tp + "/keys/1", mem, api.ShareKeyParams{Keys: []api.DeviceKey{{Device: mem.dev.ID, Sealed: sealed}}}, 403},
		{"outsider team", "GET", tp, out, nil, 404},
		{"outsider items", "GET", tp + "/items", out, nil, 404},
		{"outsider keys", "GET", tp + "/keys", out, nil, 404},
		{"outsider drops a key", "DELETE", tp + "/keys/1", out, nil, 404},
		{"outsider devices", "GET", tp + "/devices", out, nil, 404},
		{"outsider recovery", "GET", tp + "/recovery", out, nil, 404},
		{"outsider ticket", "POST", tp + "/tickets", out, api.HostTicketParams{Host: own.dev.ID}, 404},
		{"outsider audit", "GET", tp + "/audit", out, nil, 404},
		{"outsider joins via share", "PUT", tp + "/keys/1", out, api.ShareKeyParams{Keys: []api.DeviceKey{{Device: own.dev.ID, Sealed: sealed}}}, 404},
		{"member ticket into outsider's host", "POST", tp + "/tickets", mem, api.HostTicketParams{Host: "ffffffffffffffff"}, 404},
		{"owner of t into other team", "GET", op + "/items", own, nil, 404},
		{"remove other account's device", "DELETE", e.url + "/v1/devices/" + own.dev.ID, mem, nil, 404},
	}
	for _, c := range cases {
		if code, msg := send(t, c.method, c.url, c.as.token, c.body, nil); code != c.want {
			t.Errorf("%s: %d %q, want %d", c.name, code, msg, c.want)
		}
	}
}

// One removed from the default team, or who left it, isn't joined again
// by signing in.
func TestDefaultTeamKeepsOutWhoLeft(t *testing.T) {
	e := newTeamEnv(t, Config{})
	own := e.signIn(1, "own", "")
	team := e.team(own, "Everyone")
	srv := e.on(Config{DefaultTeam: team, DefaultRole: api.RoleMember}, e.db)
	bob := e.signIn(2, "bob", "")
	ann := e.signIn(3, "ann", "")
	call(t, srv, "GET", api.PathTeams, bob.token, nil)
	call(t, srv, "GET", api.PathTeams, ann.token, nil)
	if code, _ := send(t, "DELETE", srv+"/v1/teams/"+team+"/members/"+bob.acc.ID, own.token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("remove bob: %d", code)
	}
	if code, _ := send(t, "DELETE", srv+"/v1/teams/"+team+"/members/"+ann.acc.ID, ann.token, nil, nil); code != http.StatusNoContent {
		t.Fatalf("ann leaves: %d", code)
	}
	for _, m := range []signedInMember{bob, ann} {
		var list []api.Team
		call(t, srv, "GET", api.PathTeams, m.token, &list)
		if len(list) != 0 {
			t.Errorf("%s back in the default team: %+v", m.acc.GitHub, list)
		}
	}
}
