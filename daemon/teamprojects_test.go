package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A team's projects: an admin adds one by its Git link (never one with
// credentials in it), it reaches the members' hosts with the team's
// space, and a member's host that has it, as a project or a session's
// folder, finds its copy by the remote.
func TestTeamProjectsAreFoundOnTheMembersHosts(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	inv, _ := db.CreateInvite(ctx, team.ID, ada.acc, api.InviteParams{Role: api.RoleMember})
	db.AcceptInvite(ctx, inv.Token, bob.acc)
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	bob.srv.learnTeams(bob.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleMember}})

	if _, err := ada.c.AddTeamProject(proto.TeamProjectAddParams{Team: team.ID, Repo: "https://ada:ghp_secret@github.com/enowx/app.git"}); err == nil {
		t.Fatal("a link with credentials in it was added")
	}
	tp, err := ada.c.AddTeamProject(proto.TeamProjectAddParams{Team: team.ID, Repo: "git@github.com:enowx/app.git"})
	if err != nil || tp.Name != "app" || tp.Ref != "git:github.com/enowx/app" {
		t.Fatalf("added: %+v %v", tp, err)
	}
	if _, err := bob.c.AddTeamProject(proto.TeamProjectAddParams{Team: team.ID, Repo: "https://github.com/enowx/web"}); err == nil {
		t.Fatal("a member added a project")
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)
	list, err := bob.c.TeamProjects()
	if err != nil || len(list) != 1 || list[0].Ref != tp.Ref || len(list[0].Here) != 0 {
		t.Fatalf("the member's host, before it has it: %+v %v", list, err)
	}

	// Bob works on his clone in a session, the folder no project of his.
	dir := gitRepo(t)
	if out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/enowx/app").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := bob.c.CreateSession(proto.SessionCreateParams{Dir: dir, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		list, _ = bob.c.TeamProjects()
		if len(list) == 1 && len(list[0].Here) == 1 && list[0].Here[0].Path == dir && list[0].Here[0].Session != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the session's folder wasn't found as the team's project: %+v", list)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := ada.c.RemoveTeamProject(team.ID, tp.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := ada.c.TeamProjects(); len(list) != 0 {
		t.Fatalf("removed, it stayed: %+v", list)
	}
}
