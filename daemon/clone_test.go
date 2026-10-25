package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// Cloning a repository makes it a project, its progress told as events;
// a clone into a folder that has things in it is refused.
func TestCloneMakesAProject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	src := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "first"}} {
		if out, err := exec.Command("git", append([]string{"-C", src}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	srv := startServer(t)
	done := make(chan proto.CloneStatus, 8)
	c := connect(t, srv, func(e proto.Event) {
		var st proto.CloneStatus
		if e.Name == proto.EventProjectClone && json.Unmarshal(e.Data, &st) == nil && st.State != proto.CloneRunning {
			done <- st
		}
	})
	dir := t.TempDir()
	st, err := c.CloneProject(proto.ProjectCloneParams{URL: "file://" + src, Dir: dir, Name: "app", Depth: 1})
	if err != nil || st.State != proto.CloneRunning || st.Path != filepath.Join(dir, "app") {
		t.Fatalf("started %+v, %v", st, err)
	}
	select {
	case st = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the clone never ended")
	}
	if st.State != proto.CloneDone || st.Project == nil || st.Project.Path != filepath.Join(dir, "app") || !st.Project.IsGit {
		t.Fatalf("ended %+v", st)
	}
	list, _ := c.Projects()
	if len(list) != 1 || list[0].Name != "app" {
		t.Fatalf("projects %+v", list)
	}
	if got, err := c.CloneStatus(st.ID); err != nil || got.State != proto.CloneDone {
		t.Fatalf("status %+v, %v", got, err)
	}
	if _, err := c.CloneProject(proto.ProjectCloneParams{URL: "file://" + src, Dir: dir, Name: "app"}); err == nil {
		t.Fatal("cloned into a folder with things in it")
	}
	if _, err := c.CloneProject(proto.ProjectCloneParams{URL: "not a url", Dir: dir}); err == nil {
		t.Fatal("cloned an address that isn't one")
	}
}

// Where clones go and what was cloned lately is kept with the user's
// synced settings: the folder per host, the repositories for every host.
func TestClonePrefsAreKept(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	home, _ := os.UserHomeDir()
	srv.keepClone(proto.ProjectCloneParams{URL: "https://example.com/a.git", Dir: filepath.Join(home, "code")})
	srv.keepClone(proto.ProjectCloneParams{URL: "https://example.com/b.git", Dir: filepath.Join(home, "code")})
	srv.keepClone(proto.ProjectCloneParams{URL: "https://example.com/a.git", Dir: filepath.Join(home, "code")})
	p, err := c.ClonePrefs()
	if err != nil {
		t.Fatal(err)
	}
	if p.Dir != "~/code" || p.Default != "~/code" || len(p.URLs) != 2 || p.URLs[0] != "https://example.com/a.git" {
		t.Fatalf("prefs %+v", p)
	}
}
