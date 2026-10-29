package daemon

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/hostauth"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// teamHost is a host of a team's member, signed in to the cloud as a
// device of theirs.
type teamHost struct {
	srv *Server
	c   *client.Client
	acc string
}

func signInHost(t *testing.T, db *store.DB, cloudURL string, gh int64, login string) teamHost {
	t.Helper()
	ctx := context.Background()
	srv := startServer(t)
	dk, err := srv.cloud.store.DeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	acc, _ := db.GitHubAccount(ctx, gh, login, login, "", "")
	dev, token, err := db.AddDevice(ctx, acc.ID, login+"'s server", api.DeviceHost, dk.Public())
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.cloud.store.Put(hostauth.State{URL: cloudURL, Token: token, AccountID: acc.ID, Account: login, DeviceID: dev.ID}); err != nil {
		t.Fatal(err)
	}
	return teamHost{srv: srv, c: connect(t, srv, nil), acc: acc.ID}
}

func (h teamHost) sync(t *testing.T, team string) proto.Space {
	t.Helper()
	sp, err := h.srv.syncSpace(team)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	return sp
}

func (h teamHost) noteTitled(t *testing.T, title string) (proto.Note, bool) {
	t.Helper()
	list, err := h.c.Notes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range list {
		if n.Title == title {
			full, err := h.c.Note(n.ID)
			if err != nil {
				t.Fatal(err)
			}
			return full, true
		}
	}
	return proto.Note{}, false
}

// A team's space goes from one member's host to another's through the
// cloud, which keeps only ciphertext: the owner's host starts the team's
// key and seals it for the member's; changes on both sides merge; a page
// holding a secret stays home; a member removed loses the space.
func TestTeamSpaceSyncsThroughTheCloud(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")
	team, err := db.CreateTeam(ctx, ada.acc, "Design")
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := db.CreateInvite(ctx, team.ID, ada.acc, api.InviteParams{Role: api.RoleMember})
	if _, err := db.AcceptInvite(ctx, inv.Token, bob.acc); err != nil {
		t.Fatal(err)
	}
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	bob.srv.learnTeams(bob.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleMember}})

	// Bob's host, first, has no key: nobody started it.
	if _, err := bob.srv.syncSpace(team.ID); err == nil || !strings.Contains(err.Error(), "waiting for the team's key") {
		t.Fatalf("a member's host synced with no key: %v", err)
	}
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Roadmap", Team: team.ID}, Body: "one\ntwo\nthree\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Mine"}}); err != nil {
		t.Fatal(err)
	}
	if sp := ada.sync(t, team.ID); sp.SyncErr != "" || sp.Synced.IsZero() {
		t.Fatalf("the owner's sync: %+v", sp)
	}
	bob.sync(t, team.ID)
	n, ok := bob.noteTitled(t, "Roadmap")
	if !ok || n.Team != team.ID || n.Body != "one\ntwo\nthree\n" {
		t.Fatalf("the team's note on the member's host: %+v %v", n, ok)
	}
	if _, ok := bob.noteTitled(t, "Mine"); ok {
		t.Fatal("the owner's own note reached the member")
	}
	var sealed [][]byte
	rows, _ := db.QueryContext(ctx, `SELECT sealed FROM items WHERE team_id = $1`, team.ID)
	for rows.Next() {
		var b []byte
		rows.Scan(&b)
		sealed = append(sealed, b)
	}
	rows.Close()
	for _, b := range sealed {
		if bytes.Contains(b, []byte("Roadmap")) || bytes.Contains(b, []byte("three")) {
			t.Fatal("the cloud holds a note in the clear")
		}
	}

	// Both change it, apart: the changes merge.
	an, _ := ada.noteTitled(t, "Roadmap")
	an.Body = "ONE\ntwo\nthree\n"
	if _, err := ada.c.SaveNote(an); err != nil {
		t.Fatal(err)
	}
	n.Body = "one\ntwo\nTHREE\n"
	if _, err := bob.c.SaveNote(n); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID) // merges, and sends the merge
	ada.sync(t, team.ID)
	for name, h := range map[string]teamHost{"ada": ada, "bob": bob} {
		if got, _ := h.noteTitled(t, "Roadmap"); got.Body != "ONE\ntwo\nTHREE\n" {
			t.Fatalf("%s's merged note: %q", name, got.Body)
		}
	}

	// Canvases go too.
	if _, err := ada.c.CreateCanvas(proto.CanvasCreateParams{Title: "Logo", Team: team.ID}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)
	if list, err := bob.c.Canvases(); err != nil || len(list) != 1 || list[0].Title != "Logo" || list[0].Team != team.ID {
		t.Fatalf("the team's canvas on the member's host: %+v %v", list, err)
	}

	// A page holding a secret stays on the host that has it.
	if _, err := bob.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Keys", Team: team.ID}, Body: "aws AKIAABCDEFGHIJKLMNOP\n"}); err != nil {
		t.Fatal(err)
	}
	if sp := bob.sync(t, team.ID); !strings.Contains(sp.SyncErr, "secret") {
		t.Fatalf("a secret was shared, or not said: %+v", sp)
	}
	ada.sync(t, team.ID)
	if _, ok := ada.noteTitled(t, "Keys"); ok {
		t.Fatal("a page with a secret reached the team")
	}

	// Removed from the team, bob's host drops its space, his own notes stay.
	if err := db.RemoveMember(ctx, team.ID, ada.acc, bob.acc); err != nil {
		t.Fatal(err)
	}
	if _, err := bob.srv.syncSpace(team.ID); err == nil {
		t.Fatal("a removed member's host synced")
	}
	if sp, _ := bob.c.Spaces(); len(sp) != 0 {
		t.Fatalf("the space stayed: %+v", sp)
	}
}
