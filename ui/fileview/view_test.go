package fileview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

func TestTreeUnfoldsAndOpensFiles(t *testing.T) {
	v := New()
	v.loaded, v.root = true, "/work/app"
	v.dirs[""] = proto.FileListing{Root: "/work/app", Entries: []proto.FileEntry{
		{Name: "src", Dir: true, Status: "M"},
		{Name: "build", Dir: true, Ignored: true},
		{Name: "README.md", Size: 12},
	}}
	v.dirs["src"] = proto.FileListing{Entries: []proto.FileEntry{{Name: "main.go", Status: "M"}, {Name: "new.go", Status: "?"}}}
	v.layout()
	var opened []string
	o := Options{OpenFile: func(p string) { opened = append(opened, p) }}
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 480, 500)
	tt.Frame()
	if !tt.HasText("src") || !tt.HasText("README.md") || tt.HasText("main.go") {
		t.Fatalf("folded tree: %q", tt.Texts())
	}
	if err := tt.Click("src"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("main.go") || !tt.HasText("U") {
		t.Fatalf("unfolded src: %q", tt.Texts())
	}
	// Files open in tabs of their own: the tree stays.
	if err := tt.Click("src/main.go"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(opened) != 1 || opened[0] != "src/main.go" || !tt.HasText("README.md") {
		t.Fatalf("opened %q, tree %q", opened, tt.Texts())
	}
}

func TestDocShowsTheFile(t *testing.T) {
	d := NewDoc("src/main.go")
	d.content = &proto.FileContent{Path: "src/main.go", Size: 23, Text: "package main\nfunc main() {}\n"}
	d.lines = splitLines(d.content.Text)
	tt := ui.NewTester(func(c *ui.Context) { d.Draw(c, Options{Session: proto.SessionInfo{Dir: "/work/app"}}) }, 480, 300)
	tt.Frame()
	for _, want := range []string{"src/", "main.go", "package main", "func main() {}", "23 B"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
}
