package transfer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func write(t *testing.T, dir, rel, text string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A git project carries what git keeps and its history; a folder without
// git leaves dependencies out; .env files go only when asked for.
func TestProjectFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	repo := t.TempDir()
	write(t, repo, "main.go", "package main")
	write(t, repo, ".gitignore", "node_modules/\n.env\nout/\n")
	write(t, repo, "node_modules/x/index.js", "x")
	write(t, repo, "out/bin", "x")
	write(t, repo, ".env", "SECRET=1")
	write(t, repo, "new.txt", "not added yet")
	for _, args := range [][]string{{"init", "-q"}, {"add", "main.go", ".gitignore"}, {"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-qm", "x"}} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	files, err := ProjectFiles(repo, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".gitignore", "main.go", "new.txt", ".git/HEAD"} {
		if !slices.Contains(files, want) {
			t.Errorf("missing %s in %q", want, files)
		}
	}
	for _, not := range []string{".env", "node_modules/x/index.js", "out/bin"} {
		if slices.Contains(files, not) {
			t.Errorf("carried %s", not)
		}
	}
	if files, _ := ProjectFiles(repo, true); !slices.Contains(files, ".env") {
		t.Error(".env left out when asked for")
	}

	plain := t.TempDir()
	write(t, plain, "app.py", "x")
	write(t, plain, ".venv/lib/x.py", "x")
	write(t, plain, ".env.local", "x")
	files, err = ProjectFiles(plain, false)
	if err != nil || !slices.Equal(files, []string{"app.py"}) {
		t.Fatalf("plain folder: %q, %v", files, err)
	}
}

// What goes in comes out, links as links; an entry leaving the archive
// is refused.
func TestArchiveRoundTrip(t *testing.T) {
	src, dest := t.TempDir(), t.TempDir()
	write(t, src, "a/b.txt", "hello")
	if err := os.Symlink("a/b.txt", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.AddBytes(ManifestName, []byte(`{"version":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.AddFiles(ProjectPrefix, src, []string{"a/b.txt", "link"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	var names []string
	err := Read(&buf, func(e Entry) error {
		names = append(names, e.Name)
		if rel, ok := strings.CutPrefix(e.Name, ProjectPrefix+"/"); ok {
			return Extract(e, dest, rel)
		}
		_, err := io.Copy(io.Discard, e)
		return err
	})
	if err != nil || !slices.Equal(names, []string{ManifestName, "project/a/b.txt", "project/link"}) {
		t.Fatalf("read %q, %v", names, err)
	}
	if data, _ := os.ReadFile(filepath.Join(dest, "link")); string(data) != "hello" {
		t.Fatalf("through the link: %q", data)
	}

	var evil bytes.Buffer
	gz := gzip.NewWriter(&evil)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o600, Size: 1, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	if err := Read(&evil, func(Entry) error { return nil }); err == nil {
		t.Fatal("read an entry leaving the archive")
	}
}

func TestRewritePaths(t *testing.T) {
	got := string(RewritePaths([]byte(`{"cwd":"/a/app","file":"/a/app/x.go","other":"/a/app2"}`), "/a/app", "/home/u/app"))
	want := `{"cwd":"/home/u/app","file":"/home/u/app/x.go","other":"/a/app2"}`
	if got != want {
		t.Fatalf("got %s", got)
	}
}
