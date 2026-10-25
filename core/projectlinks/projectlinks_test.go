package projectlinks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/proto"
)

func TestIDStaysWithTheFolderOutOfGit(t *testing.T) {
	dir := t.TempDir()
	if ReadID(dir) != "" {
		t.Fatal("an ID before one was made")
	}
	id, err := EnsureID(dir)
	if err != nil || !ValidID(id) {
		t.Fatalf("EnsureID = %q, %v", id, err)
	}
	if again, _ := EnsureID(dir); again != id {
		t.Errorf("ID changed: %s, then %s", id, again)
	}
	moved := filepath.Join(t.TempDir(), "renamed")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	if ReadID(moved) != id {
		t.Error("the ID didn't follow the folder")
	}
	ignore, _ := os.ReadFile(filepath.Join(moved, ".enowx", "local", ".gitignore"))
	if string(ignore) != "*\n" {
		t.Errorf(".gitignore = %q", ignore)
	}
}

func link() proto.ProjectLink {
	return proto.ProjectLink{
		Project: "0123456789abcdef", Name: "antares",
		Source: proto.ProjectEnd{Daemon: "mac", Host: "Mac", Path: "/Users/me/antares"},
		Target: proto.ProjectEnd{Daemon: "vps", Host: "datalix", Path: "/home/me/antares"},
	}
}

func TestLinksOnePerPairWithStatusAndHistory(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	l, err := s.SaveLink(link())
	if err != nil || l.ID == "" {
		t.Fatalf("SaveLink = %+v, %v", l, err)
	}
	// The same copies again, the other way: the same link, updated.
	again := link()
	again.Source, again.Target = again.Target, again.Source
	again.Auto, again.Interval = true, 60
	if got, err := s.SaveLink(again); err != nil || got.ID != l.ID {
		t.Fatalf("saved again as %+v, %v", got, err)
	}
	other := link()
	other.Target.Daemon = "vps2"
	if _, err := s.SaveLink(other); err != nil {
		t.Fatal(err)
	}
	list, _ := s.Links(l.Project)
	if len(list) != 2 || !list[0].Auto {
		t.Fatalf("links = %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings", "project-links.json")); err != nil {
		t.Error("links aren't with the user's settings, which sync")
	}
	bad := link()
	bad.Direction = "sideways"
	if _, err := s.SaveLink(bad); err == nil {
		t.Error("a direction of none saved")
	}

	when := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	err = s.Report(proto.ProjectLinkReport{Link: l.ID, Status: proto.ProjectLinkStatus{Last: when, Sent: 3},
		Entry: proto.ProjectHistoryEntry{Time: when, Project: l.Project, Kind: "sync", OK: true, Summary: "3 files to datalix"}})
	if err != nil {
		t.Fatal(err)
	}
	states, _ := s.States(l.Project)
	if states[0].Status.Sent != 3 {
		t.Errorf("status = %+v", states[0].Status)
	}
	if h, _ := s.History(""); len(h) != 1 || h[0].Summary != "3 files to datalix" {
		t.Errorf("history = %+v", h)
	}
	if err := s.DeleteLink(l.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Links(""); len(list) != 1 {
		t.Errorf("after delete: %+v", list)
	}
}

func TestMatch(t *testing.T) {
	for _, c := range []struct {
		pattern, path string
		want          bool
	}{
		{"node_modules", "web/node_modules/react/index.js", true},
		{"*.log", "logs/app.log", true},
		{"*.log", "app.go", false},
		{"src/**/*.test.ts", "src/a/b/x.test.ts", true},
		{"src/**/*.test.ts", "src/x.test.ts", true},
		{"src/**/*.test.ts", "lib/x.test.ts", false},
		{"build/", "build/out/app", true},
		{"docs/*.md", "docs/a/b.md", false},
	} {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v", c.pattern, c.path, got)
		}
	}
	if Synced(".enowx/local/project.json", nil, nil) {
		t.Error("enowX's local folder synced")
	}
	if !Synced("src/a.go", []string{"src"}, []string{"*.tmp"}) || Synced("README.md", []string{"src"}, nil) {
		t.Error("include")
	}
}

func file(p, hash string, size int64) proto.SyncFile {
	return proto.SyncFile{Path: p, Hash: hash, Size: size}
}

// A sync on its own: two ways, what changed on both stays and is told;
// one way, the source's wins and the copy's new files stay; deletes and
// large files follow the link's rules.
func TestAutoPlan(t *testing.T) {
	base := &projectsync.Base{Files: map[string]map[string]string{"": {
		"a.go": "1", "both.go": "1", "gone.go": "1", "logo.png": "1", "copy-edit.go": "1",
	}}}
	src := proto.ProjectSyncState{Files: []proto.SyncFile{
		file("a.go", "2", 10), file("both.go", "2", 10), file("logo.png", "2", 10), file("copy-edit.go", "1", 10),
		file("video.mp4", "v", 80<<20),
	}}
	dst := proto.ProjectSyncState{Files: []proto.SyncFile{
		file("a.go", "1", 10), file("both.go", "3", 10), file("gone.go", "1", 10), file("logo.png", "3", 10),
		file("copy-edit.go", "2", 10), file("new-on-copy.go", "n", 10),
	}}
	plan := projectsync.Compare(base, src, dst)
	now := time.Now()
	summary := func(p projectsync.Plan) string {
		var out []string
		for _, f := range p.Files {
			s := f.Path + "<" + map[projectsync.Side]string{Source: "src", Target: "dst"}[f.From]
			if f.Delete {
				s += " deleted"
			}
			out = append(out, s)
		}
		return strings.Join(out, ", ")
	}

	l := link()
	got, conflicts := AutoPlan(l, plan, [2]proto.ProjectSyncState{src, dst}, now)
	if s := summary(got); s != "a.go<src, copy-edit.go<dst, gone.go<src deleted, new-on-copy.go<dst" {
		t.Errorf("two ways: %s", s)
	}
	if len(conflicts) != 2 || conflicts[0].Path != "both.go" || conflicts[1].Path != "logo.png" || !conflicts[1].Binary {
		t.Errorf("conflicts = %+v", conflicts)
	}

	l.Direction, l.Large, l.Deletes = proto.SyncOneWay, proto.LargeSource, proto.DeletesNever
	got, conflicts = AutoPlan(l, plan, [2]proto.ProjectSyncState{src, dst}, now)
	if s := summary(got); s != "a.go<src, both.go<src, copy-edit.go<src, logo.png<src, video.mp4<src" || len(conflicts) != 0 {
		t.Errorf("one way: %s, conflicts %v", s, conflicts)
	}
}
