package ui

import (
	"image/png"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// The New pipeline dialog starts a pipeline on the daemon, named after
// its task, and the project shows its stages.
func TestNewPipelineStartsAndShowsItsStages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	proj, err := cl.AddProject(repo)
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	app.client = cl
	app.agents = agent.List()
	app.projects = []proto.ProjectInfo{proj}
	app.openPipeline(proj)
	app.piping.task = "Add a search box to the list"
	app.piping.branch = branchFor(app.piping.task)
	if app.piping.branch != "pipeline/add-a-search-box-to" {
		t.Fatalf("branch %q", app.piping.branch)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(700).Height(800).Background(theme.Background).Children(func() { app.pipelineDialog(c) })
	}, 700, 800)
	tt.Frame()
	for _, want := range []string{"Stages", "Add a stage", "Start"} {
		if !tt.HasText(want) {
			t.Fatalf("no %q in %q", want, tt.Texts())
		}
	}
	if len(app.piping.stages) != 3 || app.piping.stages[2].Name != "Review" {
		t.Fatalf("stages %+v", app.piping.stages)
	}
	shot(t, tt, "pipeline-dialog.png")
	if err := tt.Click("Start"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for len(app.pipelines) == 0 || app.pipelines[0].Stages[0].Session == "" {
		if time.Now().After(deadline) {
			t.Fatalf("no pipeline running: %+v, form %+v", app.pipelines, app.piping)
		}
		time.Sleep(100 * time.Millisecond)
		tt.Frame()
		list, _ := cl.Pipelines()
		app.pipelines = list
	}
	cards := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(252).Height(200).Background(theme.Panel).Children(func() { app.pipelineCards(c, proj) })
	}, 252, 200)
	cards.Frame()
	for _, want := range []string{"Add a search box to the list", "Plan", "Review"} {
		if !cards.HasText(want) {
			t.Fatalf("card lacks %q: %q", want, cards.Texts())
		}
	}
	shot(t, cards, "pipeline-card.png")
}

// shot saves what tt shows when SHOTS names a folder, to look at.
func shot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, tt.Image())
}
