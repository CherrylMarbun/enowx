package cli

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An agent imports a folder of Markdown from the command line, after a
// dry run; importing it again makes nothing twice; the notes go out as a
// .zip, which another import reads back.
func TestNotesImportAndExportFromTheCommandLine(t *testing.T) {
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
	vault := t.TempDir()
	os.MkdirAll(filepath.Join(vault, "Ideas"), 0o755)
	os.WriteFile(filepath.Join(vault, "Home.md"), []byte("See [[Debloater]].\n"), 0o644)
	os.WriteFile(filepath.Join(vault, "Ideas", "Debloater.md"), []byte("- [ ] scan apps\n"), 0o644)

	if got := run("import", vault, "--dry-run"); !strings.Contains(got, "would make 2 notes") || !strings.Contains(got, "new       Debloater  in Ideas") {
		t.Fatalf("dry run:\n%s", got)
	}
	if got := run("import", vault); !strings.Contains(got, "Imported 2 notes") {
		t.Fatalf("import:\n%s", got)
	}
	if got := run(); !strings.Contains(got, "[Ideas]") || !strings.Contains(got, "Debloater") {
		t.Fatalf("tree:\n%s", got)
	}
	if got := run("import", vault); !strings.Contains(got, "Imported 0 notes") || !strings.Contains(got, "imported before") {
		t.Fatalf("again:\n%s", got)
	}
	if got := run("import-status"); strings.Count(got, proto.ImportDone) != 3 {
		t.Fatalf("status:\n%s", got)
	}
	dir := t.TempDir()
	if got := run("export", "--all", "--format", "zip", "--out", dir); !strings.Contains(got, "Exported 2 notes to "+filepath.Join(dir, "enowx-notes.zip")) {
		t.Fatalf("export:\n%s", got)
	}
	if got := run("export", "Home", "--out", dir); !strings.Contains(got, filepath.Join(dir, "Home.md")) {
		t.Fatalf("export one:\n%s", got)
	}
	t.Setenv(proto.EnvNotesPropose, "1")
	if err := (runner{c: c, out: &out}).notes([]string{"import", vault}); err == nil {
		t.Fatal("the Notes page's agent imported")
	}
}
