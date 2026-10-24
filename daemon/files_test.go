package daemon

import (
	"bytes"
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
	f, err := c.ReadFile(info.ID, "notes.md", time.Time{})
	if err != nil || f.Text != "# hi\n" {
		t.Fatalf("read = %+v, %v", f, err)
	}
	if _, err := c.ReadFile(info.ID, "../../etc/passwd", time.Time{}); err == nil {
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

func TestUploadInPartsAndCleanUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "sleep 30"}, Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	// Larger than a part, so it goes in two.
	data := make([]byte, proto.UploadPart+1234)
	for i := range data {
		data[i] = byte(i % 251)
	}
	local := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(local, data, 0o644); err != nil {
		t.Fatal(err)
	}
	remote, err := c.Upload(info.ID, local)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(remote)
	if err != nil || !bytes.Equal(got, data) || filepath.Base(remote) != "shot.png" {
		t.Fatalf("uploaded %s: %d bytes, %v", remote, len(got), err)
	}
	// The same name again is numbered, not overwritten.
	again, _ := c.Upload(info.ID, local)
	if filepath.Base(again) != "shot-2.png" {
		t.Fatalf("second upload at %s", again)
	}
	// A part can't be written elsewhere.
	if err := c.Call(proto.MethodFileUpload, proto.FileUpload{Session: info.ID, Name: "x", Path: "/tmp/x", Data: []byte("x")}, nil); err == nil {
		t.Fatal("wrote outside the session's uploads")
	}
	if err := c.KillSession(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(remote)); !os.IsNotExist(err) {
		t.Fatal("uploads outlive their session")
	}
}

// Files and folders of the app's machine go into a folder of the
// session's folder, with what a folder holds; a name taken gets a number,
// and nothing goes outside the session's folder.
func TestAddFilesIntoAFolder(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	root := t.TempDir()
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: root, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	local := t.TempDir()
	for name, content := range map[string]string{"logo.png": "png", "brand/colors.css": "a{}", "brand/fonts/inter.txt": "font"} {
		p := filepath.Join(local, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var copied int
	pick := []string{filepath.Join(local, "logo.png"), filepath.Join(local, "brand")}
	if err := c.AddFiles(info.ID, pick, "docs/assets", func(string) { copied++ }); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"logo.png": "png", "brand/colors.css": "a{}", "brand/fonts/inter.txt": "font"} {
		if data, err := os.ReadFile(filepath.Join(root, "docs", "assets", name)); err != nil || string(data) != want {
			t.Errorf("%s: %q, %v", name, data, err)
		}
	}
	if copied != 3 {
		t.Fatalf("told of %d files", copied)
	}
	if err := c.AddFiles(info.ID, pick[:1], "docs/assets", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "assets", "logo-2.png")); err != nil {
		t.Fatalf("a second logo.png: %v", err)
	}
	// A folder reaching out is taken as inside: it can't leave.
	if err := c.AddFiles(info.ID, pick[:1], "../outside", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "outside", "logo.png")); err != nil {
		t.Fatalf("not kept inside: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside")); err == nil {
		t.Fatal("a file went outside the session's folder")
	}
}
