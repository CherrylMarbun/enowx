package cli

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/hostauth"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An agent in a team's project works with the team's notes: it lists and
// reads those, and the user's own only when attached to the project; what
// it makes is the team's.
func TestAgentInATeamsProjectSeesTheTeamsNotes(t *testing.T) {
	dir := t.TempDir()
	const team = "0123456789abcdef"
	hostauth.Open(filepath.Join(dir, "cloud")).Put(hostauth.State{URL: "https://cloud.test", Token: "enxc_t", AccountID: "acc1"})
	os.MkdirAll(filepath.Join(dir, "teams", team), 0o700)
	os.WriteFile(filepath.Join(dir, "teams", team, "team.json"), []byte(`{"team":"`+team+`","name":"Design","role":"member","account":"acc1"}`), 0o600)
	srv, err := daemon.NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a, b := net.Pipe()
	go srv.ServeConn(a)
	c, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	const ref = "git:github.com/enowx/app"
	c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Diary"}, Body: "mine"})
	plan, _ := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "App plan"}, Body: "attached"})
	c.AttachNote(plan.ID, []proto.NoteProject{{Ref: ref, Name: "app", Remote: "https://github.com/enowx/app"}})
	c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Roadmap", Team: team}, Body: "the team's"})

	t.Setenv(proto.EnvTeam, team)
	t.Setenv(proto.EnvTeamProject, ref)
	var out bytes.Buffer
	run := func(args ...string) (string, error) {
		out.Reset()
		err := (runner{c: c, out: &out}).notes(args)
		return out.String(), err
	}
	tree, err := run()
	if err != nil || !strings.Contains(tree, "Roadmap") || !strings.Contains(tree, "App plan") || strings.Contains(tree, "Diary") {
		t.Fatalf("what the agent lists: %q %v", tree, err)
	}
	if _, err := run("show", "Diary"); err == nil {
		t.Fatal("the agent read the user's own note, not attached")
	}
	if body, err := run("show", "Roadmap"); err != nil || !strings.Contains(body, "the team's") {
		t.Fatalf("the team's note: %q %v", body, err)
	}
	id, err := run("new", "Findings", "--no-attach")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := c.Note(strings.TrimSpace(id)); err != nil || n.Team != team {
		t.Fatalf("the agent's new note: %+v %v", n, err)
	}
	// A canvas it makes is the team's too, so it lists and opens it.
	out.Reset()
	if err := (runner{c: c, out: &out}).canvasCmd([]string{"new", "Dashboard"}); err != nil {
		t.Fatal(err)
	}
	cv := strings.TrimSpace(out.String())
	out.Reset()
	if err := (runner{c: c, out: &out}).canvasCmd([]string{"list"}); err != nil || !strings.Contains(out.String(), cv) {
		t.Fatalf("the agent's canvases: %q %v", out.String(), err)
	}
	if got, err := c.Canvas(cv); err != nil || got.Team != team {
		t.Fatalf("the agent's new canvas: %+v %v", got.CanvasInfo, err)
	}
}
