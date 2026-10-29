package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/hostauth"
	"github.com/enowx/enowx/proto"
)

// signedInWithTeam signs srv's host in as account acc, a member of team
// "Design" (what the cloud's list of teams tells it).
func signedInWithTeam(t *testing.T, srv *Server, acc string) string {
	t.Helper()
	if err := srv.cloud.store.Put(hostauth.State{URL: "https://cloud.test", Token: "enxc_test", AccountID: acc}); err != nil {
		t.Fatal(err)
	}
	team := "0123456789abcdef"
	srv.learnTeams(acc, []api.Team{{ID: team, Name: "Design"}})
	return team
}

// A team's notes, canvases and groups live in its space, apart from the
// user's own, marked with the team in every list; pages follow their
// parent, and nothing moves between spaces by being dragged.
func TestTeamSpaceKeepsItsOwnNotesAndCanvases(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	if _, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Before", Team: "0123456789abcdef"}}); err == nil {
		t.Fatal("a team's note saved while not signed in")
	}
	team := signedInWithTeam(t, srv, "acc1")
	if sp, err := c.Spaces(); err != nil || len(sp) != 1 || sp[0].Name != "Design" {
		t.Fatalf("spaces %+v, %v", sp, err)
	}

	mine, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Mine"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SaveNoteGroup("", proto.NoteGroup{Name: "Specs", Team: team}); err != nil {
		t.Fatal(err)
	}
	theirs, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Roadmap", Team: team, Group: "Specs"}, Body: "v1"})
	if err != nil || theirs.Team != team {
		t.Fatalf("team note %+v, %v", theirs, err)
	}
	page, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Q1", Parent: theirs.ID}})
	if err != nil || page.Team != team {
		t.Fatalf("page under a team's note %+v, %v", page, err)
	}
	if _, err := os.Stat(filepath.Join(srv.dataDir, "teams", team, "notes", theirs.ID+".md")); err != nil {
		t.Fatalf("the team's note isn't in its folder: %v", err)
	}
	list, err := c.Notes()
	if err != nil {
		t.Fatal(err)
	}
	teams := map[string]string{}
	for _, n := range list {
		teams[n.Title] = n.Team
	}
	if len(list) != 3 || teams["Mine"] != "" || teams["Roadmap"] != team || teams["Q1"] != team {
		t.Fatalf("notes %+v", list)
	}
	theirs.Title = "Roadmap 2027"
	if _, err := c.SaveNote(proto.Note{NoteInfo: theirs, Body: "v2"}); err != nil {
		t.Fatal(err)
	}
	if n, err := c.Note(theirs.ID); err != nil || n.Body != "v2" || n.Team != team {
		t.Fatalf("team note read %+v, %v", n, err)
	}
	if err := c.MoveNote(mine.ID, theirs.ID, ""); err == nil {
		t.Fatal("a personal note moved under a team's")
	}
	groups, err := c.NoteGroups()
	if err != nil || len(groups) != 1 || groups[0].Team != team {
		t.Fatalf("groups %+v, %v", groups, err)
	}

	cv, err := c.CreateCanvas(proto.CanvasCreateParams{Title: "Logo", Team: team})
	if err != nil || cv.Team != team {
		t.Fatalf("team canvas %+v, %v", cv, err)
	}
	if got, err := c.Canvas(cv.ID); err != nil || got.Team != team {
		t.Fatalf("team canvas read %+v, %v", got, err)
	}
	if cl, err := c.Canvases(); err != nil || len(cl) != 1 || cl[0].Team != team {
		t.Fatalf("canvases %+v, %v", cl, err)
	}

	// Signed in as another account, the team's space is out of sight.
	if err := srv.cloud.store.Put(hostauth.State{URL: "https://cloud.test", Token: "enxc_other", AccountID: "acc2"}); err != nil {
		t.Fatal(err)
	}
	if list, err := c.Notes(); err != nil || len(list) != 1 || list[0].ID != mine.ID {
		t.Fatalf("another account's notes %+v, %v", list, err)
	}
}
