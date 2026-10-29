package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A member who lost their only device gets the team's space back on a new
// one with the recovery key they kept.
func TestRecoveryKeyBringsTheSpaceBack(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Roadmap", Team: team.ID}, Body: "kept"})
	ada.sync(t, team.ID)
	rec, err := ada.c.NewRecoveryKey()
	if err != nil || !strings.HasPrefix(rec.Key, "enxr-") {
		t.Fatalf("the recovery key: %+v %v", rec, err)
	}
	ada.sync(t, team.ID) // the team's key sealed to it too

	// The host is lost; ada signs in on a new one.
	old, _ := ada.srv.cloud.store.Get()
	db.RemoveDevice(ctx, ada.acc, old.DeviceID)
	again := signInHost(t, db, cloud.URL, 1, "ada")
	again.srv.learnTeams(again.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	if _, err := again.srv.syncSpace(team.ID); err == nil {
		t.Fatal("the new host synced with no key")
	}
	if _, err := again.c.UseRecoveryKey("enxr-NOT-A-KEY"); err == nil {
		t.Fatal("a wrong recovery key was taken")
	}
	got, err := again.c.UseRecoveryKey(rec.Key)
	if err != nil || got.Recovered != 1 {
		t.Fatalf("recovered: %+v %v", got, err)
	}
	again.sync(t, team.ID)
	if n, ok := again.noteTitled(t, "Roadmap"); !ok || n.Body != "kept" {
		t.Fatalf("the team's note on the new host: %+v", n)
	}
}
