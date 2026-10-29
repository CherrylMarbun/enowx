package daemon

import (
	"context"
	"encoding/json"
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
	"github.com/enowx/enowx/proto"
)

// A team's kit: a member's skill goes into it (one holding a secret
// doesn't), reaches the other members' hosts with the space, and a
// session in one of the team's projects is told of it.
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
	if _, err := ada.c.AddTeamProject(proto.TeamProjectAddParams{Team: team.ID, Repo: "https://github.com/enowx/app"}); err != nil {
		t.Fatal(err)
	}
	// Definitions too: a pipeline, an automation in the team's project, a
	// template; none holding a secret, an automation only in a team's project.
	stages, _ := json.Marshal([]proto.PipelineStage{{Name: "Plan", Agent: "claude", Prompt: "plan {task}"}, {Name: "Build", Agent: "codex", Prompt: "build {previous}"}})
	if _, err := ada.c.SaveToTeamKit(proto.TeamKitSaveParams{Team: team.ID, Kind: proto.KitPipeline, Name: "plan-build", About: "Plan, then build", Data: stages}); err != nil {
		t.Fatal(err)
	}
	auto, _ := json.Marshal(proto.Automation{Name: "nightly", Agent: "claude", Prompt: "update deps", Schedule: "daily 02:00", Project: "git:github.com/enowx/app"})
	if _, err := ada.c.SaveToTeamKit(proto.TeamKitSaveParams{Team: team.ID, Kind: proto.KitAutomation, Name: "nightly", Data: auto}); err != nil {
		t.Fatal(err)
	}
	stray, _ := json.Marshal(proto.Automation{Agent: "claude", Prompt: "x", Schedule: "hourly", Project: "/tmp/elsewhere"})
	if _, err := ada.c.SaveToTeamKit(proto.TeamKitSaveParams{Team: team.ID, Kind: proto.KitAutomation, Name: "stray", Data: stray}); err == nil {
		t.Fatal("an automation outside the team's projects was kept")
	}
	secretStages, _ := json.Marshal([]proto.PipelineStage{{Name: "x", Agent: "claude", Prompt: "use AKIAABCDEFGHIJKLMNOP"}})
	if _, err := ada.c.SaveToTeamKit(proto.TeamKitSaveParams{Team: team.ID, Kind: proto.KitPipeline, Name: "leaky", Data: secretStages}); err == nil {
		t.Fatal("a pipeline holding a secret was kept")
	}
	tmpl, _ := json.Marshal(proto.KitTemplateData{Repo: "git@github.com:enowx/starter.git"})
	if _, err := ada.c.SaveToTeamKit(proto.TeamKitSaveParams{Team: team.ID, Kind: proto.KitTemplate, Name: "starter", Data: tmpl}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team.ID)
	bob.sync(t, team.ID)

	kinds := map[string]string{}
	kit, _ := bob.c.TeamKit()
	for _, it := range kit {
		kinds[it.Kind] = it.Name
	}
	if len(kit) != 4 || kinds[proto.EnvSkill] != "review" || kinds[proto.KitPipeline] != "plan-build" || kinds[proto.KitAutomation] != "nightly" || kinds[proto.KitTemplate] != "starter" {
		t.Fatalf("the kit on the member's host: %+v", kit)
	}
	dir := gitRepo(t)
	exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:enowx/app.git").Run()
	guide := bob.srv.projectGuide(dir)
	if !strings.Contains(guide, "Team Design") || !strings.Contains(guide, "skill review: Review a change the team's way") ||
		!strings.Contains(guide, filepath.Join("teams", team.ID, "kit", "skills", "review", "SKILL.md")) {
		t.Fatalf("the guide of a session in the team's project: %q", guide)
	}
	if _, err := bob.c.CreateSession(proto.SessionCreateParams{Dir: dir, Cols: 80, Rows: 10,
		Command: []string{"sh", "-c", `printf '%s|%s\n' "$ENOWX_TEAM" "$ENOWX_TEAM_PROJECT" > team.txt; sleep 30`}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(waitFile(t, filepath.Join(dir, "team.txt"))); got != team.ID+"|git:github.com/enowx/app" {
		t.Fatalf("the session's team: %q", got)
	}
	if dirs := bob.srv.projectDirs([]string{"git:github.com/enowx/app", "/a/folder"}); len(dirs) != 2 || dirs[0] != dir {
		t.Fatalf("a team canvas's projects, on the member's host: %v", dirs)
	}
	if strings.Contains(guide, "plan-build") {
		t.Fatal("the guide lists the kit's definitions")
	}
	if g := bob.srv.projectGuide(gitRepo(t)); g != "" {
		t.Fatalf("a folder no team's got the team's guide: %q", g)
	}
}
