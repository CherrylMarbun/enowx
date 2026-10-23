package diffview

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestConflictBannerShowsTheOtherSession(t *testing.T) {
	v := loaded()
	shown := ""
	o := Options{
		Conflicts:   []ConflictNote{{File: "a.go", Session: "s2", With: "search", Lines: true}, {File: "b.go", Session: "s3", With: "docs"}},
		ShowSession: func(id string) { shown = id },
	}
	tt := ui.NewTester(func(c *ui.Context) { v.Draw(c, o) }, 600, 500)
	tt.Frame()
	for _, want := range []string{"2 files also changed in another session", "a.go", "· search", "same lines", "· docs"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("a.go in search"); err != nil {
		t.Fatal(err)
	}
	if shown != "s2" {
		t.Fatalf("showed %q", shown)
	}
}
