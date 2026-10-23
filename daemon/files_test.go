package daemon

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func TestFilesOfASession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "sleep 30"}, Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	l, err := c.ListFiles(info.ID, "")
	if err != nil || len(l.Entries) != 1 || l.Entries[0].Name != "notes.md" {
		t.Fatalf("listing = %+v, %v", l, err)
	}
	f, err := c.ReadFile(info.ID, "notes.md")
	if err != nil || f.Text != "# hi\n" {
		t.Fatalf("read = %+v, %v", f, err)
	}
	if _, err := c.ReadFile(info.ID, "../../etc/passwd"); err == nil {
		t.Fatal("read outside the session's folder")
	}
}

func TestPortsOfASession(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("ports aren't read on " + runtime.GOOS)
	}
	if _, err := exec.LookPath("nc"); err != nil {
		t.Skip("needs nc")
	}
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()

	srv := startServer(t)
	c := connect(t, srv, nil)
	// A stand-in for a dev server the agent started.
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "nc -l 127.0.0.1 " + strconv.Itoa(port)}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := c.Ports(info.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range got.Ports {
			if p.Port == port && p.Process == "nc" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("ports = %+v, want %d", got, port)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
