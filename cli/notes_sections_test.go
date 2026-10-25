package cli

import (
	"bytes"
	"net"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
)

// An agent reads and changes a long note a section at a time: its
// outline, one section shown, one section rewritten and added to, one
// to-do ticked, the rest of the note as it was.
func TestNotesBySection(t *testing.T) {
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
	body := "# Plan\n\n## Canvas\n\n:::tabs\n::tab Ringkasan\n- [ ] sketch\n::tab Tes\n- [ ] try it\n:::\n\n## Sync\n- [x] design\n- [ ] build\n"
	run("new", "TODO", "--body", body)

	outline := run("outline", "TODO")
	for _, want := range []string{"1/4 to-dos done\n", "#canvas    Canvas (0/2)\n", "#ringkasan      [tab] Ringkasan (0/1)\n", "#sync    Sync (1/2)\n"} {
		if !strings.Contains(outline, want) {
			t.Fatalf("outline lacks %q:\n%s", want, outline)
		}
	}
	if got := run("show", "TODO", "--section", "#sync"); got != "## Sync\n- [x] design\n- [ ] build\n" {
		t.Fatalf("show section: %q", got)
	}
	run("write", "TODO", "--section", "Tes", "--body", "- [ ] try it on a VPS")
	run("write", "TODO", "--section", "sync", "--append", "- [ ] test")
	run("check", "TODO", "--item", "build")
	got := run("show", "TODO")
	want := "# TODO\n\n# Plan\n\n## Canvas\n\n:::tabs\n::tab Ringkasan\n- [ ] sketch\n::tab Tes\n- [ ] try it on a VPS\n:::\n\n## Sync\n- [x] design\n- [x] build\n- [ ] test\n"
	if got != want {
		t.Fatalf("after:\n%q\nwant\n%q", got, want)
	}
	if err := (runner{c: c, out: &out}).notes([]string{"check", "TODO", "--item", "nothing like it"}); err == nil {
		t.Error("ticked a to-do that isn't there")
	}
}
