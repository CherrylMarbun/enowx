package notes

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestNormalRemote(t *testing.T) {
	for _, r := range []string{"git@github.com:me/app.git", "https://github.com/me/app", "ssh://git@GitHub.com/me/app.git", "https://github.com/me/app.git/"} {
		if got := NormalRemote(r); got != "github.com/me/app" {
			t.Errorf("NormalRemote(%q) = %q", r, got)
		}
	}
}

// A note attaches to projects on two hosts, keeps them through saves,
// and loses one detached, the others staying.
func TestNotesAttachToProjects(t *testing.T) {
	s := Open(t.TempDir())
	n, _ := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "Plan"}, Body: "- [ ] api\n"})
	web := proto.NoteProject{Name: "web", Host: "mac", Path: "/Users/me/web", Remote: "git@github.com:me/web.git"}
	api := proto.NoteProject{Name: "api", Host: "vps", Path: "/home/me/api"}
	cli := proto.NoteProject{Name: "cli", Host: "vps", Path: "/home/me/cli"}
	if _, err := s.Attach(n.ID, []proto.NoteProject{web, api, cli}); err != nil {
		t.Fatal(err)
	}
	full, _ := s.Get(n.ID)
	full.Body = "- [x] api\n"
	full.Projects = nil // a page saved knows nothing of its projects
	s.Save(full)
	got, _ := s.Get(n.ID)
	if len(got.Projects) != 3 || got.Projects[0].Ref != "git:github.com/me/web" || got.Projects[1].Ref != "path:vps:/home/me/api" {
		t.Fatalf("projects %+v", got.Projects)
	}
	if _, err := s.Detach(n.ID, []string{"path:vps:/home/me/api"}, false); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(n.ID)
	if len(got.Projects) != 2 || got.Body != "- [x] api\n" {
		t.Fatalf("after detaching: %+v %q", got.Projects, got.Body)
	}
	if _, err := s.Detach(n.ID, nil, true); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Get(n.ID); len(got.Projects) != 0 || got.Title != "Plan" {
		t.Fatalf("after detaching all: %+v", got)
	}
	if !IsProject(web, "laptop", proto.CatalogProject{Path: "/other/web", Remote: "https://github.com/me/web"}) {
		t.Error("the same repository cloned elsewhere isn't the same project")
	}
}
