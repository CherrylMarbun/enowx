package cli

import (
	"bytes"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
)

// An agent clones a repository as a project and waits for it; a wrong
// address comes back as an error it can read.
func TestProjectCloneFromTheCommandLine(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	src := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "first"}} {
		if out, err := exec.Command("git", append([]string{"-C", src}, args...)...).CombinedOutput(); err != nil {
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
	c, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var out bytes.Buffer
	r := runner{c: c, out: &out}
	dir := t.TempDir()
	if err := r.projectCmd([]string{"clone", "file://" + src, "--dir", dir, "--name", "web"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "now a project") {
		t.Fatalf("said %q", out.String())
	}
	out.Reset()
	if err := r.projectCmd([]string{"list"}); err != nil || strings.TrimSpace(out.String()) != filepath.Join(dir, "web") {
		t.Fatalf("list %q, %v", out.String(), err)
	}
	err = r.projectCmd([]string{"clone", "file:///no/such/repo", "--dir", dir})
	if err == nil || !strings.Contains(err.Error(), "no such repository") {
		t.Fatalf("a missing repository: %v", err)
	}
}
