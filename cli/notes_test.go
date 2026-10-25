package cli

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An agent writes the user's notes from the command line: one made under
// another, added to, shown, listed as a tree and deleted, by title or ID.
func TestNotesFromTheCommandLine(t *testing.T) {
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
	run := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := (runner{c: c, out: &out}).notes(args); err != nil {
			t.Fatalf("notes %q: %v", args, err)
		}
		return out.String()
	}
	run("new", "Release", "--icon", "🚀", "--body", "- [ ] tag")
	id := strings.TrimSpace(run("new", "Checklist", "--parent", "release"))
	run("write", id, "--append", "- [x] changelog")
	run("write", "Release", "--append", "- [ ] publish")
	if got := run("show", "Release"); got != "# 🚀 Release\n\n- [ ] tag\n- [ ] publish\n" {
		t.Fatalf("show: %q", got)
	}
	tree := run()
	if !strings.Contains(tree, "🚀 Release\n") || !strings.Contains(tree, id+"    Checklist\n") {
		t.Fatalf("tree:\n%s", tree)
	}
	run("rm", "Release")
	if got := run(); !strings.Contains(got, "No notes yet") {
		t.Fatalf("after rm:\n%s", got)
	}
}

// In a Notes page agent's session, writing a note proposes it: the note
// stays as it was until the user accepts, and deleting is refused.
func TestNotesProposedFromANotesAgent(t *testing.T) {
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
	r := runner{c: c, out: &bytes.Buffer{}}
	if err := r.notes([]string{"new", "Plan", "--body", "draft"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv(proto.EnvNotesPropose, "1")
	t.Setenv("ENOWX_SESSION", "s1")
	if err := r.notes([]string{"write", "Plan", "--body", "- [ ] tidy"}); err != nil {
		t.Fatal(err)
	}
	if err := r.notes([]string{"rm", "Plan"}); err == nil {
		t.Fatal("an agent deleted a note")
	}
	list, _ := c.Notes()
	n, _ := c.Note(list[0].ID)
	if n.Body != "draft" {
		t.Fatalf("written before accepting: %q", n.Body)
	}
	props, err := c.NoteProposals()
	if err != nil || len(props) != 1 || props[0].Before != "draft" || props[0].Note.Body != "- [ ] tidy" || props[0].Session != "s1" {
		t.Fatalf("proposals %+v, %v", props, err)
	}
	if _, err := c.AcceptNoteProposal(props[0].ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := c.Note(list[0].ID); n.Body != "- [ ] tidy" {
		t.Fatalf("after accepting: %q", n.Body)
	}
	if props, _ := c.NoteProposals(); len(props) != 0 {
		t.Fatalf("left %+v", props)
	}
}
