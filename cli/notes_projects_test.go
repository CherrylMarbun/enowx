package cli

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
)

// An agent attaches a plan to its project and another, lists a
// project's notes, reads its context, and detaches one.
func TestNotesAttachFromTheCommandLine(t *testing.T) {
	srv, err := daemon.NewServer(t.TempDir())
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
	root := t.TempDir()
	for _, name := range []string{"web", "api"} {
		dir := filepath.Join(root, name)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+name+"\nthe "+name+" part\n"), 0o644)
		if _, err := c.AddProject(dir); err != nil {
			t.Fatal(err)
		}
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if cats, _ := c.Catalogs(); len(cats) == 1 && len(cats[0].Projects) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no catalog")
		}
	}
	var out bytes.Buffer
	run := func(cmd func(runner, []string) error, args ...string) string {
		t.Helper()
		out.Reset()
		if err := cmd(runner{c: c, out: &out}, args); err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		return out.String()
	}
	run(runner.notes, "new", "Login plan", "--body", "- [ ] fix auth")
	t.Chdir(filepath.Join(root, "web"))
	got := run(runner.notes, "attach", "Login plan", "--project", ".", "--project", "api")
	if !strings.Contains(got, "web\tpath:") || !strings.Contains(got, "api\tpath:") {
		t.Fatalf("attach:\n%s", got)
	}
	if got := run(runner.notes, "--project", "api"); !strings.Contains(got, "Login plan  (also web") {
		t.Fatalf("notes --project:\n%s", got)
	}
	if got := run(runner.projectCmd, "context", "web"); !strings.Contains(got, "the web part") {
		t.Fatalf("context:\n%s", got)
	}
	if got := run(runner.projectCmd, "read", "api", "README.md"); got != "# api\nthe api part\n" {
		t.Fatalf("read: %q", got)
	}
	run(runner.notes, "detach", "Login plan", "--project", "web")
	if got := run(runner.notes, "projects", "Login plan"); strings.Contains(got, "web") || !strings.Contains(got, "api") {
		t.Fatalf("after detaching:\n%s", got)
	}
	if err := (runner{c: c, out: &out}).notes([]string{"attach", "Login plan", "--project", "nowhere"}); err == nil || !strings.Contains(err.Error(), "api (") {
		t.Errorf("an unknown project: %v", err)
	}
}
