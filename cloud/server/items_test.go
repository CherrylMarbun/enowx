package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/enowx/enowx/cloud/api"
)

// A team's items: written at their next version only, so two devices
// changing one never lose a change unseen; read back by what changed
// after the last seen; written by members, read by viewers; for no one
// else.
func TestTeamItemsGoByVersion(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{URL: "https://cloud.example.com"})
	token := func(gh int64, login string) string {
		acc, _ := db.GitHubAccount(ctx, gh, login, "", "", "")
		_, tok, _ := db.AddDevice(ctx, acc.ID, login+"'s laptop", api.DeviceApp, make([]byte, 32))
		return tok
	}
	ada, vic, eve := token(1, "ada"), token(2, "vic"), token(3, "eve")
	name := "enowX"
	var team api.Team
	send(t, "POST", srv.URL+api.PathTeams, ada, api.TeamParams{Name: &name}, &team)
	tp := srv.URL + "/v1/teams/" + team.ID
	var inv api.Invite
	send(t, "POST", tp+"/invites", ada, api.InviteParams{Role: api.RoleViewer}, &inv)
	send(t, "POST", srv.URL+api.PathJoin, vic, api.JoinParams{Token: inv.Token}, nil)
	const id = "0123456789abcdef0123456789abcdef"
	put := func(tok string, p api.PutItemParams) (int, api.Item) {
		var it api.Item
		code, _ := send(t, "PUT", tp+"/items/"+id, tok, p, &it)
		return code, it
	}
	note := func(v int64, data string) api.PutItemParams {
		return api.PutItemParams{Kind: api.ItemNote, Version: v, Epoch: 1, Sealed: []byte(data)}
	}

	if code, it := put(ada, note(1, "v1")); code != http.StatusOK || it.Seq != 1 || it.Version != 1 {
		t.Fatalf("first version: %d %+v", code, it)
	}
	if code, it := put(ada, note(1, "v1 again")); code != http.StatusConflict || it.Version != 1 || string(it.Sealed) != "v1" {
		t.Fatalf("a version taken: %d %+v", code, it)
	}
	if code, _ := put(ada, api.PutItemParams{Kind: api.ItemNote, Version: 2, Epoch: 2, Sealed: []byte("x")}); code != http.StatusBadRequest {
		t.Fatalf("sealed with an epoch not the team's: %d", code)
	}
	if code, _ := put(vic, note(2, "viewer's")); code != http.StatusForbidden {
		t.Fatalf("a viewer wrote: %d", code)
	}
	if code, _ := put(eve, note(2, "stranger's")); code != http.StatusNotFound {
		t.Fatalf("one not a member wrote: %d", code)
	}
	if code, it := put(ada, api.PutItemParams{Kind: api.ItemNote, Version: 2, Epoch: 1, Deleted: true}); code != http.StatusOK || !it.Deleted || it.Seq != 2 {
		t.Fatalf("deleted: %d %+v", code, it)
	}

	var page api.Items
	if code, _ := send(t, "GET", tp+"/items?after=0", vic, nil, &page); code != http.StatusOK || len(page.Items) != 1 || !page.Items[0].Deleted || page.Seq != 2 {
		t.Fatalf("what changed, as a viewer reads it: %d %+v", code, page)
	}
	if send(t, "GET", tp+"/items?after=2", ada, nil, &page); len(page.Items) != 0 || page.Seq != 2 {
		t.Fatalf("nothing after the last seen: %+v", page)
	}
	if code, _ := send(t, "GET", tp+"/items", eve, nil, nil); code != http.StatusNotFound {
		t.Fatalf("one not a member read: %d", code)
	}

	// Devices record what the cloud can't see: a member, an action it knows.
	if code, _ := send(t, "POST", tp+"/audit", ada, api.AuditParams{Action: api.AuditRulesChanged, Detail: "app"}, nil); code != http.StatusNoContent {
		t.Fatalf("recorded: %d", code)
	}
	if code, _ := send(t, "POST", tp+"/audit", vic, api.AuditParams{Action: api.AuditRulesChanged}, nil); code != http.StatusForbidden {
		t.Fatalf("a viewer recorded: %d", code)
	}
	if code, _ := send(t, "POST", tp+"/audit", ada, api.AuditParams{Action: "team.deleted"}, nil); code != http.StatusBadRequest {
		t.Fatalf("an action the cloud records itself: %d", code)
	}
	var n int
	db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit WHERE team_id = $1 AND action = $2`, team.ID, api.AuditRulesChanged).Scan(&n)
	if n != 1 {
		t.Fatalf("audit rows: %d", n)
	}
}
