package daemon

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/proto"
)

func writeT(t *testing.T, path, text string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readT(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

// A project linked to its copy on another host syncs on its own: what
// changed on one side goes to the other, what changed on both stays as
// each has it and is told, and the link and its status are on both hosts.
func TestLinkSyncsCopiesOnItsOwn(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	srcSrv, dstSrv := startServer(t), startServer(t)
	src, dst := connect(t, srcSrv, nil), connect(t, dstSrv, nil)
	srcInfo, _ := src.Info()
	dstInfo, _ := dst.Info()
	if srcInfo.ID == "" || srcInfo.ID == dstInfo.ID {
		t.Fatalf("daemons' IDs: %q, %q", srcInfo.ID, dstInfo.ID)
	}
	repo := filepath.Join(t.TempDir(), "app")
	writeT(t, filepath.Join(repo, "main.go"), "package main\n")
	writeT(t, filepath.Join(repo, "shared.txt"), "one\n")
	writeT(t, filepath.Join(repo, "debug.log"), "noise\n")
	if _, err := src.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	id, err := src.ProjectID(repo)
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "app-copy")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.AddProject(dest); err != nil {
		t.Fatal(err)
	}
	l := proto.ProjectLink{
		Project: id.ID, Name: "app", Auto: true, Interval: 60, Exclude: []string{"*.log"},
		Source: proto.ProjectEnd{Daemon: srcInfo.ID, Host: "Mac", Path: repo},
		Target: proto.ProjectEnd{Daemon: dstInfo.ID, Host: "vps", Path: dest},
	}
	// The app records it on both hosts (with the user's data, it would
	// sync there too).
	if l, err = src.SaveProjectLink(l); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.SaveProjectLink(l); err != nil {
		t.Fatal(err)
	}
	if found, err := dst.FindProject(id.ID); err != nil || found.Path != dest {
		t.Fatalf("the copy isn't found by the project's ID: %+v, %v", found, err)
	}

	run := func(base *projectsync.Base) projectlinks.Result {
		t.Helper()
		res, err := projectlinks.Sync{
			Link: l, Daemons: [2]projectlinks.Daemon{src, dst}, Base: base,
			Carry: func(from projectsync.Side, id string, size int64) error {
				c := [2]*client.Client{src, dst}
				return client.CopyTransfer(c[from], c[from.Other()], id, size, nil)
			},
		}.Do()
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	res := run(nil)
	if res.Sent != 2 || readT(filepath.Join(dest, "main.go")) != "package main\n" {
		t.Fatalf("first sync: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dest, "debug.log")); err == nil {
		t.Error("an excluded file went")
	}

	// Each side works; both change shared.txt.
	writeT(t, filepath.Join(dest, "server.go"), "package main // on the server\n")
	writeT(t, filepath.Join(repo, "main.go"), "package main\n\nfunc main() {}\n")
	writeT(t, filepath.Join(repo, "shared.txt"), "mac\n")
	writeT(t, filepath.Join(dest, "shared.txt"), "vps\n")
	res = run(&res.Base)
	if res.Sent != 1 || res.Received != 1 || len(res.Conflicts) != 1 || res.Conflicts[0].Path != "shared.txt" {
		t.Fatalf("second sync: %+v", res)
	}
	if readT(filepath.Join(repo, "server.go")) == "" || !strings.Contains(readT(filepath.Join(dest, "main.go")), "func main") {
		t.Error("the work didn't cross")
	}
	if readT(filepath.Join(repo, "shared.txt")) != "mac\n" || readT(filepath.Join(dest, "shared.txt")) != "vps\n" {
		t.Error("a file in conflict was overwritten")
	}
	// Both versions kept, the other's beside: then the conflict is gone.
	rel, err := dst.KeepProjectCopy(proto.ProjectKeepCopyParams{Project: dest, Path: "shared.txt", Suffix: "vps"})
	if err != nil || rel != "shared.conflict-vps.txt" {
		t.Fatalf("keep copy: %q, %v", rel, err)
	}

	base, _ := json.Marshal(res.Base)
	report := proto.ProjectLinkReport{Link: l.ID, Base: base, Status: proto.ProjectLinkStatus{Last: time.Now(), Conflicts: res.Conflicts},
		Entry: proto.ProjectHistoryEntry{Project: l.Project, Kind: "sync", Link: l.ID, OK: true, Summary: res.Summary(l)}}
	if err := dst.ReportProjectLink(report); err != nil {
		t.Fatal(err)
	}
	states, _ := dst.ProjectLinks(id.ID)
	if len(states) != 1 || len(states[0].Status.Conflicts) != 1 {
		t.Fatalf("links on the copy's host: %+v", states)
	}
	if raw, err := dst.ProjectLinkBase(l.ID); err != nil || len(raw) == 0 {
		t.Errorf("base not kept: %v", err)
	}
	if h, _ := dst.ProjectHistory(id.ID); len(h) != 1 || !strings.Contains(h[0].Summary, "in conflict") {
		t.Errorf("history = %+v", h)
	}
}

// Watched, a project's files changing are told once they settle.
func TestWatchTellsFilesChanged(t *testing.T) {
	srv := startServer(t)
	dir := t.TempDir()
	got := make(chan string, 4)
	c := connect(t, srv, func(ev proto.Event) {
		if ev.Name == proto.EventProjectFilesChanged {
			var ref proto.ProjectRef
			_ = json.Unmarshal(ev.Data, &ref)
			got <- ref.Path
		}
	})
	if err := c.WatchProject(dir, false); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	writeT(t, filepath.Join(dir, "a.txt"), "changed")
	select {
	case p := <-got:
		if p != dir {
			t.Errorf("told of %s", p)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("never told")
	}
	if err := c.WatchProject(dir, true); err != nil {
		t.Fatal(err)
	}
}

// An agent's "copy this project to the VPS" goes to the app attached, the
// only one reaching other hosts, and comes back with its answer.
func TestCopyRequestGoesToTheApp(t *testing.T) {
	srv := startServer(t)
	dir := t.TempDir()
	cli := connect(t, srv, nil)
	if _, err := cli.CopyProjectToHost(proto.ProjectCopyParams{Project: dir, To: "vps"}); err == nil || !strings.Contains(err.Error(), "open it") {
		t.Fatalf("no app: %v", err)
	}
	var app *client.Client
	var mu sync.Mutex
	app = connect(t, srv, func(ev proto.Event) {
		if ev.Name != proto.EventAppRequest {
			return
		}
		var req proto.AppRequest
		_ = json.Unmarshal(ev.Data, &req)
		var p proto.ProjectCopyParams
		_ = json.Unmarshal(req.Params, &p)
		res, _ := json.Marshal(proto.ProjectCopyResult{Path: "/srv/" + filepath.Base(p.Project)})
		go func() {
			mu.Lock()
			defer mu.Unlock()
			_ = app.ReplyToApp(proto.AppReply{ID: req.ID, Result: res})
		}()
	})
	if err := app.Call(proto.MethodBrowserServe, nil, nil); err != nil {
		t.Fatal(err)
	}
	res, err := cli.CopyProjectToHost(proto.ProjectCopyParams{Project: dir, To: "vps"})
	if err != nil || res.Path != "/srv/"+filepath.Base(dir) {
		t.Fatalf("copy = %+v, %v", res, err)
	}
}

// Turned on, a project's work is pushed by this host to its branch alone,
// and the settings, with the user's data, name this host as the pusher.
func TestAutopushThroughTheDaemon(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	root := t.TempDir()
	repo, remote := filepath.Join(root, "app"), filepath.Join(root, "remote.git")
	gitIn := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=a"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	writeT(t, filepath.Join(repo, "main.go"), "package main\n")
	gitIn(root, "init", "-q", "--bare", remote)
	gitIn(repo, "init", "-q", "-b", "main")
	gitIn(repo, "add", ".")
	gitIn(repo, "commit", "-qm", "first")
	gitIn(repo, "remote", "add", "origin", remote)
	writeT(t, filepath.Join(repo, "wip.go"), "package main // wip\n")

	srv := startServer(t)
	c := connect(t, srv, nil)
	if _, err := c.AddProject(repo); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetAutopush(proto.AutopushSettings{Path: repo, On: true, Branch: "main"}); err == nil {
		t.Fatal("autopush to main turned on")
	}
	st, err := c.SetAutopush(proto.AutopushSettings{Path: repo, On: true, Interval: 900})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := c.Info()
	if !st.Here || st.Host != info.ID || st.Branch != "enowx/autosync" {
		t.Fatalf("status = %+v", st)
	}
	data, _ := os.ReadFile(filepath.Join(srv.persist.dir, "settings", "autopush.json"))
	if !strings.Contains(string(data), info.ID) {
		t.Errorf("settings = %s", data)
	}
	st, err = c.AutopushNow(repo)
	if err != nil {
		t.Fatal(err)
	}
	if got := gitIn(remote, "rev-parse", "refs/heads/enowx/autosync"); got != st.Commit {
		t.Errorf("remote at %s, pushed %s", got, st.Commit)
	}
	if gitIn(repo, "status", "--porcelain") != "?? wip.go" || gitIn(repo, "branch", "--show-current") != "main" {
		t.Error("the user's work changed")
	}
	list, _ := c.Autopushes()
	if len(list) != 1 || list[0].LastPush.IsZero() {
		t.Errorf("list = %+v", list)
	}
	h, _ := c.ProjectHistory("")
	if len(h) != 1 || h[0].Kind != "push" || !h[0].OK {
		t.Errorf("history = %+v", h)
	}
}
