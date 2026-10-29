package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// A team's kit: a member's skill goes into it (one holding a secret
// doesn't), reaches the other members' hosts with the space, and a
// session in one of the team's projects is told of it; the team's
// switches set its sessions' features.
func TestTeamKitReachesSessionsInTheTeamsProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	skill := filepath.Join(home, ".claude", "skills", "review")
	os.MkdirAll(skill, 0o700)
	os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: review\ndescription: Review a change the team's way\n---\n\nRead the diff first.\n"), 0o600)
	os.WriteFile(filepath.Join(skill, "checklist.md"), []byte("- tests\n"), 0o600)
	leaky := filepath.Join(home, ".claude", "skills", "deploy")
	os.MkdirAll(leaky, 0o700)
	os.WriteFile(filepath.Join(leaky, "SKILL.md"), []byte("---\ndescription: deploy\n---\ntoken ghp_"+strings.Repeat("b", 36)+"\n"), 0o600)

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

	it, err := ada.c.AddToTeamKit(team.ID, "claude/skill/review")
	if err != nil || it.Name != "review" || it.About != "Review a change the team's way" {
		t.Fatalf("the skill added: %+v %v", it, err)
	}
	if _, err := ada.c.AddToTeamKit(team.ID, "claude/skill/deploy"); err == nil {
		t.Fatal("a skill holding a secret went to the team")
	}
	var feature string
	for _, f := range agent.Features {
		if !f.Core {
			feature = f.ID
			break
		}
	}
	if _, err := bob.c.TeamFeatures(team.ID, map[string]bool{feature: false}, true); err == nil {
		t.Fatal("a member set the team's features")
	}
	if _, err := ada.c.TeamFeatures(team.ID, map[string]bool{feature: false}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ada.c.AddTeamProject(proto.TeamProjectAddParams{Team: team.ID, Repo: "https://github.com/enowx/app"}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)

	if kit, _ := bob.c.TeamKit(); len(kit) != 1 || kit[0].Name != "review" {
		t.Fatalf("the kit on the member's host: %+v", kit)
	}
	dir := gitRepo(t)
	exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:enowx/app.git").Run()
	guide := bob.srv.projectGuide(dir)
	if !strings.Contains(guide, "Team Design") || !strings.Contains(guide, "skill review: Review a change the team's way") ||
		!strings.Contains(guide, filepath.Join("teams", team.ID, "kit", "skills", "review", "SKILL.md")) {
		t.Fatalf("the guide of a session in the team's project: %q", guide)
	}
	if g := bob.srv.projectGuide(gitRepo(t)); g != "" {
		t.Fatalf("a folder no team's got the team's guide: %q", g)
	}
	r, _, _ := bob.srv.resolveFeatures(dir, nil)
	if r[feature].On || r[feature].From != "team" {
		t.Fatalf("the team's switch in its project: %+v", r[feature])
	}
}
