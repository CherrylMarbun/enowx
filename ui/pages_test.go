package ui

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// An HTML file of a session shows as a page that loads its styles and
// fonts, whatever their size or bytes, through the daemon, and nothing
// above its folder.
func TestPagesLoadWhatSitsBesideThem(t *testing.T) {
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
	dir := t.TempDir()
	font := bytes.Repeat([]byte{0, 1, 2, 3}, proto.BytesPart/2+10) // binary, in three parts
	for name, data := range map[string][]byte{
		"site/index.html":    []byte(`<link rel="stylesheet" href="css/app.css">`),
		"site/css/app.css":   []byte("body{color:red}"),
		"site/fonts/a.woff2": font,
		"secret.txt":         []byte("not for pages"),
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	info, err := cl.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "cat"}, Dir: dir, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	app.client = cl
	page, err := app.pageURL(info, "site/index.html")
	if err != nil {
		t.Fatal(err)
	}
	get := func(url string) (int, string, []byte) {
		t.Helper()
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("Content-Type"), body
	}
	base := strings.TrimSuffix(page, "index.html")
	if code, kind, body := get(page); code != 200 || !strings.HasPrefix(kind, "text/html") || !bytes.Contains(body, []byte("app.css")) {
		t.Fatalf("page: %d %s %q", code, kind, body)
	}
	if code, kind, _ := get(base + "css/app.css"); code != 200 || !strings.HasPrefix(kind, "text/css") {
		t.Fatalf("style: %d %s", code, kind)
	}
	if code, _, body := get(base + "fonts/a.woff2"); code != 200 || !bytes.Equal(body, font) {
		t.Fatalf("font: %d, %d bytes of %d", code, len(body), len(font))
	}
	if code, _, body := get(base + "../secret.txt"); code == 200 && bytes.Contains(body, []byte("not for pages")) {
		t.Fatal("a page read above its folder")
	}
	wrong := strings.Replace(page, pages.token, strings.Repeat("0", len(pages.token)), 1)
	if code, _, _ := get(wrong); code != http.StatusNotFound {
		t.Fatalf("without the token: %d", code)
	}
}
