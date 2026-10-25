package cli

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An agent sees where a project came from, sets its copy's sync, turns
// its auto pushes on and pushes when its work is done; asking for what
// needs another host without the app open says so.
func TestProjectLinksFromTheCommandLine(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	repo, remote := filepath.Join(root, "web"), filepath.Join(root, "remote.git")
	git := func(dir string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v\n%s", err, out)
		}
	}
	_ = os.MkdirAll(repo, 0o755)
	_ = os.WriteFile(filepath.Join(repo, "index.html"), []byte("<h1>hi</h1>\n"), 0o644)
	git(root, "init", "-q", "--bare", remote)
	git(repo, "init", "-q", "-b", "main")
	git(repo, "add", ".")
	git(repo, "commit", "-qm", "first")
	git(repo, "remote", "add", "origin", remote)

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
	if _, err := c.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	info, _ := c.Info()
	id, _ := c.ProjectID(repo)
	if _, err := c.SaveProjectLink(proto.ProjectLink{Project: id.ID, Name: "web",
		Source: proto.ProjectEnd{Daemon: "0123456789abcdef", Host: "Mac", Path: "/Users/me/web"},
		Target: proto.ProjectEnd{Daemon: info.ID, Host: "vps", Path: repo}}); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := r.projectCmd(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	if got := run("origin", "web"); !strings.Contains(got, "Came from: Mac:/Users/me/web") {
		t.Errorf("origin = %q", got)
	}
	if got := run("sync", repo, "--on", "--interval", "2m", "--exclude", "*.log, tmp"); !strings.Contains(got, "on its own, every 2m0s") {
		t.Errorf("sync = %q", got)
	}
	links, _ := c.ProjectLinks(id.ID)
	if len(links) != 1 || !links[0].Auto || links[0].Interval != 120 || len(links[0].Exclude) != 2 {
		t.Errorf("link = %+v", links)
	}
	if err := r.projectCmd([]string{"sync", repo, "--now"}); err == nil || !strings.Contains(err.Error(), "open it") {
		t.Errorf("sync now without the app: %v", err)
	}
	if got := run("autopush", repo, "--on", "--interval", "15m"); !strings.Contains(got, "Auto push: on, every 15m0s, to enowx/autosync, by this host") {
		t.Errorf("autopush = %q", got)
	}
	_ = os.WriteFile(filepath.Join(repo, "wip.html"), []byte("<p>wip</p>\n"), 0o644)
	if got := run("autopush", repo, "--now"); !strings.Contains(got, "Last pushed") {
		t.Errorf("autopush now = %q", got)
	}
	if err := r.projectCmd([]string{"autopush", repo, "--branch", "main"}); err == nil {
		t.Error("autopush to main allowed")
	}
	if got := run("history", repo); !strings.Contains(got, "push\tpushed") {
		t.Errorf("history = %q", got)
	}
}
