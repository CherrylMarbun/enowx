package ui

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// An agent copies a project to another host through the app; the copy
// remembers its source, syncs on its own once turned on, keeps what
// changed on both sides as each has it until the user picks, and the
// project's work is pushed to a branch of its own.
func TestProjectLinksCopySyncAndPush(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ENOWX_HOME", t.TempDir())
	write := func(path, text string) {
		t.Helper()
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) string {
		data, _ := os.ReadFile(path)
		return string(data)
	}
	git := func(dir string, args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	root := t.TempDir()
	repo, remote := filepath.Join(root, "app"), filepath.Join(root, "remote.git")
	write(filepath.Join(repo, "main.go"), "package main\n")
	write(filepath.Join(repo, "notes.md"), "notes\n")
	git(root, "init", "-q", "--bare", remote)
	git(repo, "init", "-q", "-b", "main")
	git(repo, "add", ".")
	git(repo, "commit", "-qm", "init")
	git(repo, "remote", "add", "origin", remote)

	app := New(Options{})
	posted := make(chan func(), 256)
	app.onMain = func(fn func()) { posted <- fn }
	target := hosts.Host{ID: "vps", Name: "enowxlabs"}
	w := &hostWatch{host: target, status: hostStatus{state: hostOnline}, stop: make(chan struct{})}
	serve := func(onEvent func(proto.Event)) (*daemon.Server, *client.Client) {
		srv, err := daemon.NewServer(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(srv.Close)
		a, b := net.Pipe()
		go srv.ServeConn(a)
		c, err := client.New(b, onEvent)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return srv, c
	}
	hereSrv, here := serve(func(ev proto.Event) { app.handleEvent(0, ev) })
	_, there := serve(func(ev proto.Event) { app.watchEvent(w, ev) })
	hereInfo, _ := here.Info()
	thereInfo, _ := there.Info()
	app.client, app.host, app.daemonInfo, app.hostList = here, LocalHost(), hereInfo, []hosts.Host{target}
	w.cl, w.info = there, thereInfo
	app.watches = map[string]*hostWatch{target.ID: w}
	for _, c := range []*client.Client{here, there} {
		if err := c.ServeApp(); err != nil {
			t.Fatal(err)
		}
	}
	proj, err := here.AddProject(repo)
	if err != nil {
		t.Fatal(err)
	}

	var tt *ui.Tester
	drain := func(until func() bool, what string) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for !until() {
			select {
			case fn := <-posted:
				fn()
			case <-time.After(30 * time.Millisecond):
			}
			if tt != nil {
				tt.Frame()
			}
			if time.Now().After(deadline) {
				texts := []string(nil)
				if tt != nil {
					texts = tt.Texts()
				}
				t.Fatalf("never: %s (texts %q)", what, texts)
			}
		}
	}

	// An agent on this host asks for a copy on the VPS.
	a2 := net.Conn(nil)
	a2, b2 := net.Pipe()
	go hereSrv.ServeConn(a2)
	agent, err := client.New(b2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer agent.Close()
	dest := filepath.Join(t.TempDir(), "app")
	copied := make(chan proto.ProjectCopyResult, 1)
	go func() {
		res, err := agent.CopyProjectToHost(proto.ProjectCopyParams{Project: repo, To: "enowxlabs", Path: dest})
		if err != nil {
			t.Error(err)
		}
		copied <- res
	}()
	var res proto.ProjectCopyResult
	drain(func() bool {
		select {
		case res = <-copied:
			return true
		default:
			return false
		}
	}, "the copy")
	if res.Path != dest || read(filepath.Join(dest, "main.go")) != "package main\n" {
		t.Fatalf("copy = %+v", res)
	}
	if links, _ := there.ProjectLinks(""); len(links) != 1 || links[0].Source.Path != repo || links[0].Target.Daemon != thereInfo.ID {
		t.Fatalf("the copy's host doesn't know its source: %+v", links)
	}

	app.openLinks(proj)
	tt = ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(760).Height(820).Background(theme.Background).TextColor(theme.Text).Children(func() { app.linksDialog(c) })
	}, 760, 820)
	f := app.linksForm
	drain(func() bool { return f.pushRead && tt.HasText("Copied to enowxlabs") }, "the dialog")
	drain(func() bool { return tt.HasText("Syncs when you say · never synced") }, "the link")
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "links-copies.png")

	if err := tt.Click("Sync on its own"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool {
		links, _ := there.ProjectLinks("")
		return len(links) == 1 && links[0].Auto
	}, "auto sync saved on the copy's host")

	// Worked on in both places; notes.md on both.
	write(filepath.Join(dest, "server.go"), "package main // there\n")
	write(filepath.Join(repo, "notes.md"), "notes, here\n")
	write(filepath.Join(dest, "notes.md"), "notes, there\n")
	app.autoSyncTick()
	drain(func() bool { return tt.HasText("Changed on both since the last sync: keep whose?") }, "the sync on its own")
	if read(filepath.Join(repo, "server.go")) == "" {
		t.Error("the copy's new file didn't come")
	}
	if read(filepath.Join(repo, "notes.md")) != "notes, here\n" || read(filepath.Join(dest, "notes.md")) != "notes, there\n" {
		t.Error("a file in conflict was overwritten")
	}
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "links-conflict.png")

	if err := tt.Click("Compare"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return tt.HasText("- notes, here") && tt.HasText("+ notes, there") }, "the comparison")
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "links-compare.png")
	if err := tt.Click("Keep both"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool {
		return read(filepath.Join(repo, "notes.conflict-enowxlabs.md")) == "notes, there\n" && !tt.HasText("keep whose?")
	}, "both kept")
	if read(filepath.Join(dest, "notes.md")) != "notes, here\n" {
		t.Errorf("the copy's notes.md = %q", read(filepath.Join(dest, "notes.md")))
	}

	// Turned off, it syncs no more.
	if err := tt.Click("Sync on its own"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { links, _ := there.ProjectLinks(""); return !links[0].Auto }, "auto sync off")
	write(filepath.Join(repo, "later.go"), "package main // later\n")
	app.links.read = time.Time{}
	app.autoSyncTick()
	for range 20 {
		select {
		case fn := <-posted:
			fn()
		case <-time.After(20 * time.Millisecond):
		}
	}
	if read(filepath.Join(dest, "later.go")) != "" {
		t.Error("synced while off")
	}
	// The copy's folder renamed there and opened again: the link follows.
	moved := filepath.Join(filepath.Dir(dest), "app-renamed")
	if err := os.Rename(dest, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := there.AddProject(moved); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Sync now"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return read(filepath.Join(moved, "later.go")) != "" }, "the sync to the renamed folder")
	if links, _ := there.ProjectLinks(""); links[0].Target.Path != moved {
		t.Errorf("the link kept %s", links[0].Target.Path)
	}

	// Auto pushes, from this host.
	if err := tt.Click("Autopush"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return tt.HasText("Push the work on its own") }, "the autopush tab")
	if err := tt.Click("Auto push"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return f.push.On && f.push.Here }, "auto push on")
	if err := tt.Click("Push now"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return !f.push.LastPush.IsZero() }, "the push")
	out, err := exec.Command("git", "-C", remote, "rev-parse", "refs/heads/enowx/autosync").Output()
	if err != nil || strings.TrimSpace(string(out)) != f.push.Commit {
		t.Errorf("remote's branch = %q, %v; pushed %s", out, err, f.push.Commit)
	}
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "links-push.png")
	if err := tt.Click("History"); err != nil {
		t.Fatal(err)
	}
	drain(func() bool { return tt.HasText("Push · pushed") }, "the history")
	tt.Advance(time.Second)
	tt.Frame()
	shot(t, tt, "links-history.png")
}
