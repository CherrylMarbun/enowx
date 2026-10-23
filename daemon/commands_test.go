package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A project's command runs typed into a shell in a new tab of the session,
// and comes first among its commands after.
func TestRunCommandInATab(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"dev":"vite"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	parent, err := c.CreateSession(proto.SessionCreateParams{Command: catCommand(), Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	list, err := c.ProjectCommands(parent.ID)
	if err != nil || len(list) != 1 || list[0].Command != "npm run dev" {
		t.Fatalf("commands %+v, %v", list, err)
	}
	tab, err := c.RunCommand(parent.ID, proto.RunCommand{Name: "hello", Command: "echo ran-$((40+2))"})
	if err != nil {
		t.Fatal(err)
	}
	if tab.Parent != parent.ID || tab.Name != "hello" || tab.Dir != parent.Dir {
		t.Fatalf("tab %+v", tab)
	}
	st, err := c.Attach(tab.ID)
	if err != nil {
		t.Fatal(err)
	}
	watch(st).waitFor(t, "ran-42")
	list, err = c.ProjectCommands(parent.ID)
	if err != nil || len(list) != 2 || list[0].Source != proto.CommandRecent || list[0].Name != "hello" {
		t.Fatalf("after running: %+v, %v", list, err)
	}
}
