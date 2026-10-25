package cli

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
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
