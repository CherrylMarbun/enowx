package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
)

// project.read and project.context stay in the shared project: neither a
// link in it nor ".." leads a visitor out to the host's other files.
func TestVisitorProjectReadStaysInTheShare(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	w := newVisitWorld(t, api.RoleViewer, false)
	os.WriteFile(filepath.Join(w.private, "secret.txt"), []byte("PRIVATE_PROJECT_SECRET\n"), 0o600)
	os.WriteFile(filepath.Join(w.private, "README.md"), []byte("PRIVATE_README\n"), 0o600)
	os.WriteFile(filepath.Join(w.shared, "ok.txt"), []byte("shared\n"), 0o600)
	os.Symlink(filepath.Join(w.private, "secret.txt"), filepath.Join(w.shared, "link.txt"))
	os.Symlink(w.private, filepath.Join(w.shared, "outside"))
	v := w.visit(t)

	var got proto.ProjectFile
	if err := v.Call(proto.MethodProjectRead, proto.ProjectReadParams{Path: w.shared, File: "ok.txt"}, &got); err != nil || got.Text != "shared\n" {
		t.Fatalf("a shared file: %+v %v", got, err)
	}
	for _, p := range []proto.ProjectReadParams{
		{Path: w.private, File: "secret.txt"},
		{Path: w.shared, File: "link.txt"},
		{Path: w.shared, File: "outside/secret.txt"},
		{Path: filepath.Join(w.shared, "outside"), File: "secret.txt"},
		{Path: w.shared + "/../" + filepath.Base(w.private), File: "secret.txt"},
	} {
		var got proto.ProjectFile
		if err := v.Call(proto.MethodProjectRead, p, &got); err == nil || strings.Contains(got.Text, "PRIVATE") {
			t.Errorf("read %s in %s: %+v", p.File, p.Path, got)
		}
	}
	for _, path := range []string{filepath.Join(w.shared, "outside"), w.shared + "/../" + filepath.Base(w.private)} {
		var c proto.ProjectContext
		if err := v.Call(proto.MethodProjectContext, proto.ProjectContextParams{Path: path}, &c); err == nil || strings.Contains(c.Readme, "PRIVATE") {
			t.Errorf("the context of %s: %+v", path, c)
		}
	}
}

// Unsharing the team's last project closes its visitors' connections: no
// view keeps streaming, no takeover still types in.
func TestVisitorUnshareClosesTheConnection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	w := newVisitWorld(t, api.RoleMember, false)
	info, err := w.ada.c.CreateSession(proto.SessionCreateParams{Name: "ada's shell", Dir: w.shared, Cols: 100, Rows: 20, Command: []string{"sh"}})
	if err != nil {
		t.Fatal(err)
	}
	v := w.visit(t)
	st, err := v.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ada.c.AnswerTakeover(info.ID, w.bob.acc, true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ada.c.SetHostShare(w.team, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-v.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("unshared, the visitor stayed connected")
	}
	st.Write([]byte("echo TYPED_AFTER_UNSHARE_$((6*7))\n"))
	time.Sleep(300 * time.Millisecond)
	sess, _ := w.ada.srv.sessions.Get(info.ID)
	if strings.Contains(sess.Terminal().Text(), "TYPED_AFTER_UNSHARE_42") {
		t.Fatal("unshared, the visitor typed into the host's session")
	}
	waitFor(t, "the takeover ended", func() bool { return len(w.ada.srv.takeover.all()) == 0 })
}

// Taking back one project of two closes visitors' views of its sessions
// and ends their control of them; the connection stays for the other.
func TestVisitorUnshareOneProjectDropsItsSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	w := newVisitWorld(t, api.RoleMember, false)
	w.ada.c.SetHostShare(w.team, []string{w.shared, w.private})
	info, err := w.ada.c.CreateSession(proto.SessionCreateParams{Name: "in private", Dir: w.private, Cols: 100, Rows: 20, Command: []string{"sh"}})
	if err != nil {
		t.Fatal(err)
	}
	v := w.visit(t)
	st, err := v.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ada.c.AnswerTakeover(info.ID, w.bob.acc, true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ada.c.SetHostShare(w.team, []string{w.shared}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the view and control taken", func() bool {
		list, _ := w.ada.c.HostVisitors()
		return len(list) == 1 && len(list[0].Watching) == 0 && len(list[0].Controls) == 0
	})
	st.Write([]byte("echo TYPED_AFTER_UNSHARE_$((6*7))\n"))
	time.Sleep(300 * time.Millisecond)
	sess, _ := w.ada.srv.sessions.Get(info.ID)
	if strings.Contains(sess.Terminal().Text(), "TYPED_AFTER_UNSHARE_42") {
		t.Fatal("unshared, the visitor typed into the host's session")
	}
	if list, err := v.Sessions(); err != nil || len(list) != 0 {
		t.Fatalf("still connected, the visitor sees: %+v %v", list, err)
	}
}

// A member removed from the team can't start a session, and is let go at
// the next recheck; one made a viewer is one from then on.
func TestVisitorRemovedOrDemotedLosesAccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	old := visitorRecheck
	visitorRecheck = 200 * time.Millisecond
	t.Cleanup(func() { visitorRecheck = old })
	t.Setenv("HOME", t.TempDir())
	fakeClaude(t, `echo started`)
	w := newVisitWorld(t, api.RoleMember, true)
	ctx := context.Background()

	v := w.visit(t)
	if err := w.db.SetRole(ctx, w.team, w.ada.acc, w.bob.acc, api.RoleViewer); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the visitor made a viewer", func() bool {
		list, _ := w.ada.c.HostVisitors()
		return len(list) == 1 && list[0].Role == api.RoleViewer
	})
	if _, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: w.shared, Cols: 100, Rows: 10}); err == nil {
		t.Fatal("made a viewer, they started a session")
	}

	if err := w.db.SetRole(ctx, w.team, w.ada.acc, w.bob.acc, api.RoleMember); err != nil {
		t.Fatal(err)
	}
	if err := w.db.RemoveMember(ctx, w.team, w.ada.acc, w.bob.acc); err != nil {
		t.Fatal(err)
	}
	select {
	case <-v.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("removed from the team, the visitor stayed connected")
	}
}

// A removed member's next session is refused at once, before any recheck.
func TestVisitorRemovedCantStartASession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	t.Setenv("HOME", t.TempDir())
	fakeClaude(t, `echo started`)
	w := newVisitWorld(t, api.RoleMember, true)
	v := w.visit(t)
	if err := w.db.RemoveMember(context.Background(), w.team, w.ada.acc, w.bob.acc); err != nil {
		t.Fatal(err)
	}
	if info, err := v.CreateSession(proto.SessionCreateParams{Agent: "claude", Project: w.shared, Cols: 100, Rows: 10}); err == nil {
		t.Fatalf("a removed member started session %s", info.ID)
	}
}

// A visitor can't tell a session they don't see from one that isn't
// there, and sees nothing of the host's home, name or process, or how a
// session was started.
func TestVisitorSeesNoMoreThanShared(t *testing.T) {
	w := newVisitWorld(t, api.RoleViewer, false)
	w.ada.c.CreateSession(proto.SessionCreateParams{Name: "acquisition-talks", Dir: w.private, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})
	w.ada.c.CreateSession(proto.SessionCreateParams{Name: "shared work", Dir: w.shared, Prompt: "the host's prompt", Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})
	v := w.visit(t)
	_, e1 := v.PeekSession(proto.SessionPeekParams{Session: "acquisition-talks"})
	_, e2 := v.PeekSession(proto.SessionPeekParams{Session: "no-such-name"})
	if e1 == nil || e2 == nil || strings.ReplaceAll(e1.Error(), "acquisition-talks", "X") != strings.ReplaceAll(e2.Error(), "no-such-name", "X") {
		t.Fatalf("a hidden session and a missing one answer apart: %v / %v", e1, e2)
	}
	info, err := v.Info()
	if err != nil || info.Home != "" || info.Hostname != "" || info.PID != 0 || info.Version == "" {
		t.Fatalf("daemon.info for a visitor: %+v %v", info, err)
	}
	list, err := v.Sessions()
	if err != nil || len(list) != 1 || list[0].Command != nil || list[0].Prompt != "" {
		t.Fatalf("the sessions a visitor sees: %+v %v", list, err)
	}
}

// A visitor asks to take over a session once, until answered.
func TestVisitorTakeoverAsksAreLimited(t *testing.T) {
	w := newVisitWorld(t, api.RoleMember, false)
	info, err := w.ada.c.CreateSession(proto.SessionCreateParams{Name: "pair", Dir: w.shared, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}})
	if err != nil {
		t.Fatal(err)
	}
	v := w.visit(t)
	if err := v.AskTakeover(info.ID); err != nil {
		t.Fatal(err)
	}
	if err := v.AskTakeover(info.ID); err == nil {
		t.Fatal("asked twice, unanswered")
	}
	if err := w.ada.c.AnswerTakeover(info.ID, w.bob.acc, false); err != nil {
		t.Fatal(err)
	}
	if err := v.AskTakeover(info.ID); err != nil {
		t.Fatalf("answered, asking again: %v", err)
	}
}
