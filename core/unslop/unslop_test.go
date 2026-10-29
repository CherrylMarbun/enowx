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
