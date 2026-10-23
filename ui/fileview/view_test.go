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

func TestMarkdownRendersWithRawAtHand(t *testing.T) {
	d := NewDoc("docs/api.md")
	d.content = &proto.FileContent{Path: "docs/api.md", Size: 40, Text: "# API contract\n\nMoney is **always** a string.\n"}
	d.lines = splitLines(d.content.Text)
	tt := ui.NewTester(func(c *ui.Context) { d.Draw(c, Options{}) }, 600, 400)
	tt.Frame()
	if !tt.HasText("Preview") || !tt.HasText("Raw") || tt.HasText("# API contract") {
		t.Fatalf("preview: %q", tt.Texts())
	}
	if !tt.HasText("API contract") {
		t.Fatalf("no heading in %q", tt.Texts())
	}
	if err := tt.Click("Raw"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("# API contract") {
		t.Fatalf("raw: %q", tt.Texts())
	}
	// Other files have no such choice.
	other := NewDoc("main.go")
	other.content = &proto.FileContent{Text: "package main\n"}
	other.lines = splitLines(other.content.Text)
	tt2 := ui.NewTester(func(c *ui.Context) { other.Draw(c, Options{}) }, 600, 300)
	tt2.Frame()
	if tt2.HasText("Preview") {
		t.Fatal("a Go file offers a preview")
	}
}
