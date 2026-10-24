package ui

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

func TestPastedFilesAreDroppedPaths(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "my shot (1).png")
	b := filepath.Join(dir, "notes.md")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	paste := escapePath(a) + " " + escapePath(b)
	for _, bracketed := range []bool{false, true} {
		p := []byte(paste)
		if bracketed {
			p = append(append(append([]byte{}, pasteStart...), p...), pasteEnd...)
		}
		paths, rewrite, ok := pastedFiles(p)
		if !ok || !slices.Equal(paths, []string{a, b}) {
			t.Fatalf("bracketed %v: %q, %v", bracketed, paths, ok)
		}
		got := string(rewrite([]string{"/home/enowx/up/my shot (1).png", "/home/enowx/up/notes.md"}))
		want := `/home/enowx/up/my\ shot\ \(1\).png /home/enowx/up/notes.md`
		if bracketed {
			want = "\x1b[200~" + want + "\x1b[201~"
		}
		if got != want {
			t.Fatalf("rewritten %q, want %q", got, want)
		}
	}
	// Text that isn't only existing files is pasted as it is.
	for _, text := range []string{"hello world", escapePath(a) + " missing.txt", dir, "ls " + escapePath(b), escapePath(a) + "\n"} {
		if _, _, ok := pastedFiles([]byte(text)); ok {
			t.Errorf("%q taken for files", text)
		}
	}
}

func TestDroppedFileReachesTheServersSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	a, b := net.Pipe()
	go srv.ServeConn(a)
	cl, err := client.New(b, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	// cat shows what it is typed.
	info, err := cl.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "cat"}, Dir: t.TempDir(), Cols: 120, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	st, err := cl.Attach(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	conn := &uploadConn{
		Stream: st,
		upload: func(path string) (string, error) { return cl.Upload(info.ID, path) },
		report: func(string, bool) {},
	}
	local := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(local, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte(escapePath(local))); err != nil {
		t.Fatal(err)
	}
	// The screen's text, rows joined: a long path wraps.
	var screen string
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(screen, "/uploads/"+info.ID+"/photo.png") {
		if time.Now().After(deadline) {
			t.Fatalf("the session shows %q", screen)
		}
		time.Sleep(100 * time.Millisecond)
		text, err := cl.Screen(info.ID)
		if err != nil {
			t.Fatal(err)
		}
		var rows []string
		for _, row := range strings.Split(text, "\n") {
			rows = append(rows, strings.TrimRight(row, " "))
		}
		screen = strings.Join(rows, "")
	}
	if strings.Contains(screen, local) {
		t.Fatal("the local path reached the session")
	}
}
