package files

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func byName(l proto.FileListing) map[string]proto.FileEntry {
	out := map[string]proto.FileEntry{}
	for _, e := range l.Entries {
		out[e.Name] = e
	}
	return out
}

func TestListMarksWhatGitSays(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	write(t, filepath.Join(root, "a.txt"), "a\n")
	write(t, filepath.Join(root, "src/b.go"), "package b\n")
	write(t, filepath.Join(root, ".gitignore"), "build/\n")
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "init")
	write(t, filepath.Join(root, "a.txt"), "changed\n")
	write(t, filepath.Join(root, "src/new.go"), "package b\n")
	write(t, filepath.Join(root, "build/out"), "x")

	l, err := List(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if l.Entries[0].Name != "build" || !l.Entries[0].Dir || l.Entries[1].Name != "src" {
		t.Fatalf("folders don't come first: %+v", l.Entries)
	}
	got := byName(l)
	if _, ok := got[".git"]; ok {
		t.Fatal("lists .git")
	}
	if got["a.txt"].Status != "M" || got["a.txt"].Size != 8 {
		t.Fatalf("a.txt = %+v", got["a.txt"])
	}
	if got["src"].Status != "M" || !got["build"].Ignored || got[".gitignore"].Status != "" {
		t.Fatalf("entries = %+v", l.Entries)
	}
	sub, err := List(root, "src")
	if err != nil {
		t.Fatal(err)
	}
	if s := byName(sub); s["new.go"].Status != "?" || s["b.go"].Status != "" {
		t.Fatalf("src = %+v", sub.Entries)
	}
}

func TestPathsStayInTheFolder(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "in.txt"), "inside\n")
	write(t, filepath.Join(filepath.Dir(root), "secret.txt"), "outside\n")
	for _, p := range []string{"../secret.txt", "a/../../secret.txt"} {
		if c, err := Read(root, p, time.Time{}); err == nil && strings.Contains(c.Text, "outside") {
			t.Fatalf("read %s outside the folder", p)
		}
	}
	// A path that looks absolute is taken from the folder.
	c, err := Read(root, "/in.txt", time.Time{})
	if err != nil || c.Text != "inside\n" {
		t.Fatalf("read /in.txt: %+v, %v", c, err)
	}
}

func TestReadTellsBinaryAndLongFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "bin"), "a\x00b")
	write(t, filepath.Join(root, "long.txt"), strings.Repeat("x", maxRead+10))
	if c, _ := Read(root, "bin", time.Time{}); !c.Binary || c.Text != "" {
		t.Fatalf("bin = %+v", c)
	}
	if c, _ := Read(root, "long.txt", time.Time{}); !c.Truncated || len(c.Text) != maxRead {
		t.Fatalf("long.txt truncated %v, %d bytes", c.Truncated, len(c.Text))
	}
}

func TestReadSendsImagesAndSkipsUnchanged(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "shot.png"), buf.String())
	c, err := Read(root, "shot.png", time.Time{})
	if err != nil || c.Image != "png" || !bytes.Equal(c.Data, buf.Bytes()) || c.Binary {
		t.Fatalf("png = %+v, %v", c, err)
	}
	// Asked unless it changed since: nothing comes.
	again, err := Read(root, "shot.png", c.Modified)
	if err != nil || !again.Unchanged || again.Data != nil {
		t.Fatalf("unchanged = %+v, %v", again, err)
	}
}

// A file a program printed reads wherever it is, by its absolute path.
func TestReadAbs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shot.txt")
	write(t, p, "printed\n")
	c, err := ReadAbs(p, time.Time{})
	if err != nil || c.Text != "printed\n" || c.Path != p {
		t.Fatalf("read %+v, %v", c, err)
	}
	if _, err := ReadAbs("shot.txt", time.Time{}); err == nil {
		t.Fatal("read a relative path")
	}
}
