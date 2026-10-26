package fileview

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

func TestDocCopiesTextAndPathSeparately(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		content    proto.FileContent
	}{
		{"line endings", "src/main.go", proto.FileContent{Text: "package main\r\n// café 世界\r\n"}},
		{"empty", "empty.txt", proto.FileContent{}},
		{"Markdown preview", "README.md", proto.FileContent{Text: "# Heading\n\n**bold**\n"}},
		{"HTML preview", "index.html", proto.FileContent{Text: "<h1>Hello</h1>\n"}},
		{"truncated", "large.txt", proto.FileContent{Text: "first part\n", Truncated: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDoc(tc.file)
			d.content, d.lines = &tc.content, splitLines(tc.content.Text)
			d.page = d.html
			o := Options{Session: proto.SessionInfo{Dir: "/srv/project"}, Page: func(c *ui.Context, _ string, _ time.Time) { ui.Text(c, "Page preview") }}
			tt := ui.NewTester(func(c *ui.Context) { d.Draw(c, o) }, 720, 300)
			tt.Frame()
			tt.SetClipboard("previous clipboard")
			label := "Copy text"
			if tc.content.Truncated {
				label = "Copy displayed text"
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			if got := tt.Clipboard(); got != tc.content.Text {
				t.Fatalf("copied %q, want %q", got, tc.content.Text)
			}
			if err := tt.Click("Copy the path"); err != nil {
				t.Fatal(err)
			}
			if got := tt.Clipboard(); got != "/srv/project/"+tc.file {
				t.Fatalf("copied path %q", got)
			}
		})
	}
}

func TestDocDoesNotCopyUnreadableText(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content *proto.FileContent
		err     string
	}{
		{name: "loading"},
		{name: "binary", content: &proto.FileContent{Binary: true}},
		{name: "image", content: &proto.FileContent{Image: "svg"}},
		{name: "read error", content: &proto.FileContent{Text: "stale"}, err: "file no longer exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDoc("file")
			d.content, d.err = tc.content, tc.err
			tt := ui.NewTester(func(c *ui.Context) { d.Draw(c, Options{}) }, 480, 300)
			tt.Frame()
			if tt.HasText("Copy text") || tt.HasText("Copy displayed text") {
				t.Fatal("unreadable file offers copying text")
			}
		})
	}
}

func TestDocCopiesContentReadThroughDaemon(t *testing.T) {
	root := t.TempDir()
	want := "original source\r\nwith a trailing newline\r\n"
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte(want), 0o600); err != nil {
		t.Fatal(err)
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
	// A portable short-lived program gives the daemon a session folder.
	s, err := cl.CreateSession(proto.SessionCreateParams{Command: []string{os.Args[0], "-test.run=^$"}, Dir: root, Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	f, err := cl.ReadFile(s.ID, "source.txt", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	d := NewDoc("source.txt")
	d.content, d.lines = &f, splitLines(f.Text)
	tt := ui.NewTester(func(c *ui.Context) { d.Draw(c, Options{Session: s}) }, 480, 300)
	tt.Frame()
	if err := tt.Click("Copy text"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != want {
		t.Fatalf("daemon content copied as %q, want %q", got, want)
	}
}
