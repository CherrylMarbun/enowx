package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// An agent asked to look at another session reads its last messages and
// where its project is, a line that looks like a secret taken out; with
// no app open, the sessions listed are this host's.
func TestAgentPeeksAtAnotherSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	sess, err := srv.sessions.Create(session.Spec{Command: []string{"sleep", "30"}, Dir: dir, Meta: session.Meta{Agent: "claude", Name: "Login form"}})
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(home, ".claude", "projects", claudeFolderOf(dir))
	os.MkdirAll(folder, 0o755)
	lines := `{"type":"user","message":{"content":"build the login form"}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Done: LoginForm.tsx\nthe key: AKIAABCDEFGHIJKLMNOP"}]}}` + "\n"
	os.WriteFile(filepath.Join(folder, "s.jsonl"), []byte(lines), 0o600)
	time.Sleep(lookEvery + 100*time.Millisecond)

	all, err := c.SessionsEverywhere()
	if err != nil || len(all) != 1 || len(all[0].Sessions) != 1 || all[0].Sessions[0].ID != sess.ID() {
		t.Fatalf("the sessions everywhere, with no app: %+v %v", all, err)
	}
	p, err := c.PeekSession(proto.SessionPeekParams{Session: "login form"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Session.ID != sess.ID() || p.Session.Dir != dir || len(p.Messages) != 2 {
		t.Fatalf("the peek: %+v", p)
	}
	if p.Messages[0].Text != "build the login form" || !strings.Contains(p.Messages[1].Text, "LoginForm.tsx") {
		t.Fatalf("its messages: %+v", p.Messages)
	}
	if strings.Contains(p.Messages[1].Text, "AKIA") || !strings.Contains(p.Messages[1].Text, "taken out") {
		t.Fatalf("a secret came through: %q", p.Messages[1].Text)
	}
	if p, _ := c.PeekSession(proto.SessionPeekParams{Session: sess.ID(), Last: 1}); len(p.Messages) != 1 || p.Earlier != 1 {
		t.Fatalf("the last message only: %+v", p)
	}
	if _, err := c.PeekSession(proto.SessionPeekParams{Session: "nope"}); err == nil {
		t.Fatal("peeked at no session")
	}
}
