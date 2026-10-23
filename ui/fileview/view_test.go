package fileview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

func TestTreeUnfoldsAndOpensAFile(t *testing.T) {
	v := New()
	v.loaded, v.root = true, "/work/app"
	v.dirs[""] = proto.FileListing{Root: "/work/app", Entries: []proto.FileEntry{
		{Name: "src", Dir: true, Status: "M"},
		{Name: "build", Dir: true, Ignored: true},
		{Name: "README.md", Size: 12},
	}}
	v.dirs["src"] = proto.FileListing{Entries: []proto.FileEntry{{Name: "main.go", Status: "M"}, {Name: "new.go", Status: "?"}}}
	v.layout()
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, Options{}) }, 480, 500)
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

	// Opening a file shows its lines; the poller isn't running, so the
	// content is set as the daemon would answer.
	if err := tt.Click("README.md"); err != nil {
		t.Fatal(err)
	}
	v.reading, v.content = false, &proto.FileContent{Path: "README.md", Size: 12, Text: "# app\nhello\n"}
	v.lines = splitLines(v.content.Text)
	tt.Frame()
	if !tt.HasText("hello") || !tt.HasText("# app") {
		t.Fatalf("file: %q", tt.Texts())
	}
	if err := tt.Click("All files"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("main.go") {
		t.Fatalf("back at the tree: %q", tt.Texts())
	}
}
