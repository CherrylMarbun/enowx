//go:build !windows

package ui

import (
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/hostauth"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A team's project's pipeline dialog starts from the team's pipelines,
// and shares its stages back with the team.
func TestPipelineDialogUsesTheTeamsPipelines(t *testing.T) {
	dir := t.TempDir()
	const team = "0123456789abcdef"
	hostauth.Open(filepath.Join(dir, "cloud")).Put(hostauth.State{URL: "https://cloud.test", Token: "enxc_test", AccountID: "acc1", Account: "Ada"})
	os.MkdirAll(filepath.Join(dir, "teams", team), 0o700)
	os.WriteFile(filepath.Join(dir, "teams", team, "team.json"), []byte(`{"team":"`+team+`","name":"Design","role":"owner","account":"acc1"}`), 0o600)
	srv, err := daemon.NewServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	app := New(Options{})
	var mu sync.Mutex
	var later []func()
	app.onMain = func(fn func()) { mu.Lock(); later = append(later, fn); mu.Unlock() }
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, func(ev proto.Event) { app.handleEvent(1, ev) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	app.gen, app.client = 1, cl
	app.host = hosts.Host{ID: hosts.LocalID, Name: "this Mac"}

	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://github.com/enowx/app"}} {
		exec.Command("git", append([]string{"-C", repo}, args...)...).Run()
	}
	project, err := cl.AddProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cl.AddTeamProject(proto.TeamProjectAddParams{Team: team, Repo: "https://github.com/enowx/app"}); err != nil {
		t.Fatal(err)
	}
	stages, _ := json.Marshal([]proto.PipelineStage{{Name: "Spec", Agent: "claude", Prompt: "spec {task}"}, {Name: "Ship", Agent: "claude", Prompt: "ship {previous}"}})
	if _, err := cl.SaveToTeamKit(proto.TeamKitSaveParams{Team: team, Kind: proto.KitPipeline, Name: "spec-ship", Data: stages}); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.Window())
		ui.Column(c).Width(900).Height(800).Background(theme.Background).TextColor(theme.Text).Children(func() { app.pipelineDialog(c) })
	}, 900, 800)
	settle := func(what string, done func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			tt.Frame()
			if done() {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s: never; shows %q", what, tt.Texts())
			}
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			fns := later
			later = nil
			mu.Unlock()
			for _, fn := range fns {
				fn()
			}
		}
	}
	app.teamProjectsOf()
	app.teamKitOf()
	app.openPipeline(project)
	settle("the team's pipelines", func() bool { return tt.HasText("Design's pipelines") && tt.HasText("spec-ship") })
	if err := tt.Click("spec-ship"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(app.piping.stages) != 2 || app.piping.stages[0].Name != "Spec" {
		t.Fatalf("the stages loaded: %+v", app.piping.stages)
	}
	app.piping.shareName = "my-take"
	tt.Frame()
	if err := tt.Click("Share with Design"); err != nil {
		t.Fatal(err)
	}
	settle("shared", func() bool {
		kit, _ := cl.TeamKit()
		for _, it := range kit {
			if it.Name == "my-take" && it.Kind == proto.KitPipeline {
				return true
			}
		}
		return false
	})
}
