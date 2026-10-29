package daemon

import (
	"bytes"
	"context"
	"image"
	"image/png"
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

// Notes and canvases go between spaces: the user's own to a team's with
// the pages under them and their images, never one holding a secret; out
// of a team's only once its admins allow it, as copies of their own.
func TestNotesAndCanvasesMoveBetweenSpaces(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})

	plan, _ := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Plan"}, Body: "the plan\n"})
	var img bytes.Buffer
	png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	ref, err := ada.c.AddNoteImage(plan.ID, "shot.png", img.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	plan.Title = "Plan"
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: plan, Body: "the plan\n![shot](" + ref + ")\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Step 1", Parent: plan.ID}}); err != nil {
		t.Fatal(err)
	}
	moved, err := ada.c.MoveNoteToSpace(plan.ID, team.ID, false)
	if err != nil || moved.Team != team.ID {
		t.Fatalf("moved to the team: %+v %v", moved, err)
	}
	list, _ := ada.c.Notes()
	for _, n := range list {
		if n.Team != team.ID {
			t.Fatalf("a page stayed behind: %+v", n)
		}
	}
	if len(list) != 2 {
		t.Fatalf("the pages: %+v", list)
	}
	if data, err := ada.c.NoteAsset(ref); err != nil || !bytes.Equal(data, img.Bytes()) {
		t.Fatalf("its image: %q %v", data, err)
	}

	keys, _ := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Keys"}, Body: "ghp_" + strings.Repeat("a", 36) + "\n"})
	if _, err := ada.c.MoveNoteToSpace(keys.ID, team.ID, true); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("a note with a secret went to the team: %v", err)
	}

	// Out of the team: refused until its admins allow it, then a copy.
	if _, err := ada.c.MoveNoteToSpace(plan.ID, "", true); err == nil {
		t.Fatal("copied out of a team that keeps its items")
	}
	yes := true
	if err := db.UpdateTeam(ctx, team.ID, ada.acc, api.TeamParams{CopyOut: &yes}); err != nil {
		t.Fatal(err)
	}
	mine, err := ada.c.MoveNoteToSpace(plan.ID, "", true)
	if err != nil || mine.Team != "" || mine.ID == plan.ID {
		t.Fatalf("copied out: %+v %v", mine, err)
	}
	if n, err := ada.c.Note(mine.ID); err != nil || !strings.Contains(n.Body, "assets/"+mine.ID+"/") {
		t.Fatalf("the copy names its own image: %+v %v", n, err)
	}

	cv, _ := ada.c.CreateCanvas(proto.CanvasCreateParams{Title: "Logo"})
	got, err := ada.c.MoveCanvasToSpace(cv.ID, team.ID, false)
	if err != nil || got.Team != team.ID || got.ID != cv.ID {
		t.Fatalf("the canvas moved: %+v %v", got, err)
	}
	copied, err := ada.c.MoveCanvasToSpace(cv.ID, "", true)
	if err != nil || copied.Team != "" || copied.ID == cv.ID || copied.Title != "Logo" {
		t.Fatalf("the canvas copied out: %+v %v", copied, err)
	}
}
