package browserview

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
)

// The toolbar's Unslop starts one on a page of the project running here,
// and is off for a page elsewhere; a run under way shows over the page.
func TestUnslopInTheBrowser(t *testing.T) {
	page := &fakePage{}
	o := options(page, new(string))
	started := 0
	o.Unslop = func() { started++ }
	v := New()
	v.loaded = true
	v.setPorts(o, []proto.Port{{Port: 5173}})
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 1000, 600)
	tt.Frame()
	if err := tt.Click("Unslop: find what makes this page look AI-made, and have the session's agent fix it in the code, pass by pass"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if started != 1 {
		t.Fatalf("started %d times", started)
	}
	o.UnslopRun = &proto.CanvasUnslopRun{ID: "s1", Frame: "page", From: 8, Score: 8, Scans: 1, Passes: []string{"01 Truth", "02 Structure"}}
	tt.Frame()
	if !tt.HasText("Reading the page…") || !tt.HasText("Truth") {
		t.Fatalf("no panel over the page: %q", tt.Texts())
	}
	page.url = "https://example.com/"
	o.UnslopRun = nil
	tt.Frame()
	if err := tt.Click("Unslop works on the project's pages running here (localhost)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if started != 1 {
		t.Fatal("an unslop started on a page elsewhere")
	}
}
