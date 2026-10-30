package canvas

import (
	"strings"
	"testing"
)

// Leaving a step out puts back what it changed, unless changed since,
// and takes out what it added.
func TestLeaveOut(t *testing.T) {
	before := `<style>.a{padding:13px}</style><div id="hero"><h1 id="t">Hi</h1></div><p id="p">old</p>`
	after := `<style>.a{padding:16px}</style><div id="hero"><h1 id="t">Hello</h1></div><p id="p">new</p><span id="badge">New</span>`
	current := `<style>.a{padding:16px}</style><div id="hero"><h1 id="t">Hello</h1></div><p id="p">newer</p><span id="badge">New</span>`
	page, back, kept, err := LeaveOut(current, before, after)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"padding:13px", ">Hi<", ">newer<"} {
		if !strings.Contains(page, want) {
			t.Fatalf("no %q in %s", want, page)
		}
	}
	if strings.Contains(page, "badge") {
		t.Fatalf("what the step added stayed: %s", page)
	}
	if strings.Join(kept, ",") != "p" || len(back) != 3 {
		t.Fatalf("back %v kept %v", back, kept)
	}
}
