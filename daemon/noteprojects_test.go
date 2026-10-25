package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/usersync"
	"github.com/enowx/enowx/proto"
)

// A note attached on one host to a project of each host keeps both
// through a sync; its project's context is read where the project is,
// and from the synced snapshot when that host can't be reached.
func TestNotesAttachToProjectsOfTwoHosts(t *testing.T) {
	mac, vps := startServer(t), startServer(t)
	cm, cv := connect(t, mac, nil), connect(t, vps, nil)
	web := filepath.Join(t.TempDir(), "web")
	os.MkdirAll(filepath.Join(web, "src"), 0o755)
	os.WriteFile(filepath.Join(web, "README.md"), []byte("# Web\nThe shop's front.\n"), 0o644)
	os.WriteFile(filepath.Join(web, "package.json"), []byte(`{"dependencies":{"react":"19"}}`), 0o644)
	os.WriteFile(filepath.Join(web, "src", "auth.ts"), []byte("export const login = 1\n"), 0o644)
	if _, err := cm.AddProject(web); err != nil {
		t.Fatal(err)
	}
	mi, _ := cm.Info()
	vi, _ := cv.Info()
	n, err := cm.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Login plan"}, Body: "- [ ] fix auth\n"})
	if err != nil {
		t.Fatal(err)
	}
	both := []proto.NoteProject{
		{Name: "web", Host: mi.ID, HostName: "Mac", Path: web},
		{Name: "api", Host: vi.ID, HostName: "vps", Path: "/srv/api", Remote: "git@github.com:me/api.git"},
	}
	if _, err := cm.AttachNote(n.ID, both); err != nil {
		t.Fatal(err)
	}
	ctx, err := cm.ProjectContext(proto.ProjectContextParams{Path: web})
	if err != nil || !strings.Contains(ctx.Readme, "shop's front") || !strings.Contains(strings.Join(ctx.Stack, ","), "react") || ctx.Host != mi.ID {
		t.Fatalf("context %+v, %v", ctx, err)
	}
	if f, err := cm.ReadProjectFile(proto.ProjectReadParams{Path: web, File: "src/auth.ts"}); err != nil || f.Text != "export const login = 1\n" {
		t.Fatalf("read %+v, %v", f, err)
	}
	if _, err := usersync.Sync(client.SyncSide{C: cm}, client.SyncSide{C: cv}, &usersync.Base{}); err != nil {
		t.Fatal(err)
	}
	got, err := cv.Note(n.ID)
	if err != nil || len(got.Projects) != 2 || got.Projects[1].Ref != "git:github.com/me/api" {
		t.Fatalf("on the vps: %+v, %v", got.Projects, err)
	}
	// From the vps, the Mac's project with no app to reach it: the
	// snapshot synced from the Mac.
	cached, err := cv.ProjectContext(proto.ProjectContextParams{Path: web, Host: mi.ID, Ref: ctx.Ref})
	if err != nil || !cached.Cached || cached.Readme != ctx.Readme {
		t.Fatalf("cached context %+v, %v", cached, err)
	}
	if _, err := cv.DetachNote(n.ID, []string{got.Projects[0].Ref}, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := cv.Note(n.ID); len(got.Projects) != 1 || got.Body != "- [ ] fix auth\n" {
		t.Fatalf("after detaching one: %+v", got)
	}
}
