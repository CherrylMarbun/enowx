package unslop

import (
	"os"
	"slices"
	"testing"

	"github.com/enowx/enowx/proto"
)

func rules(list []proto.UnslopFinding) []string {
	var out []string
	for _, f := range list {
		out = append(out, f.Rule)
	}
	return out
}

// A dashboard with the tells (violet gradients, emoji, bold Arial, “!!”,
// glows, pills, off-grid spacing) is found out, pass by pass, and scores
// low; the same dashboard made with care scores high, nothing found.
func TestScanTellsSlopFromCare(t *testing.T) {
	slop, _ := os.ReadFile("testdata/slop.html")
	score, found := Scan(string(slop), proto.CanvasMockup)
	got := rules(found)
	for _, want := range []string{"fake-data", "side-stripe", "generic-cta", "spacing-off-grid", "purple-gradient", "gradients", "gradient-text", "glow", "emoji",
		"default-font", "bold-everywhere", "proportional", "exclamation", "hype", "greeting", "pills", "radius", "border-shadow", "motion", "contrast", "centered"} {
		if want == "proportional" {
			want = "tabular-numbers"
		}
		if !slices.Contains(got, want) {
			t.Errorf("not found: %s (found %v)", want, got)
		}
	}
	for _, f := range found {
		if f.Rule == "purple-gradient" && !slices.Contains(f.Targets, ".nav a.on") {
			t.Errorf("the gradient's elements: %v", f.Targets)
		}
	}
	if score > 20 {
		t.Errorf("the slop scores %d", score)
	}
	// In pass order: truth first, access last.
	if found[0].Pass != "01 Truth" || found[len(found)-1].Pass != "11 Access" {
		t.Errorf("order: first %s, last %s", found[0].Pass, found[len(found)-1].Pass)
	}

	clean, _ := os.ReadFile("testdata/clean.html")
	score, found = Scan(string(clean), proto.CanvasMockup)
	if score < 90 {
		t.Errorf("the careful design scores %d: %v", score, rules(found))
	}
}

// The theme is read from the page's background, or its text; a redesign
// that turns it is found out.
func TestTheme(t *testing.T) {
	cases := map[string]string{
		`<style>body{background:#0b0b0c;color:#eee}</style>`:             Dark,
		`<style>html,body{background-color:#fafafa}</style>`:             Light,
		`<style>body{color:#f5f5f5}</style>`:                             Dark,
		`<style>.app{background:linear-gradient(#000,#111)}</style><p>x`: Dark,
		`<p>no styles`: "",
	}
	for html, want := range cases {
		if got := Theme(html); got != want {
			t.Errorf("%s: %q, want %q", html, got, want)
		}
	}
	if _, ok := ThemeFinding(Dark, `<style>body{background:#fff}</style>`); !ok {
		t.Error("a dark design turned light wasn't found")
	}
	if _, ok := ThemeFinding(Dark, `<style>body{background:#111}</style>`); ok {
		t.Error("a dark design kept dark was found")
	}
}
