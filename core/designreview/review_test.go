package designreview

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A verdict as a reviewer writes it reads back; below the bar the agent
// is told what to fix, at the last round to stop.
func TestVerdict(t *testing.T) {
	r, err := Parse("Score: 6/10\n- Redo: the hero is a template split\nIssue: nav: three CTAs → one\nImprove: footer: drop the newsletter box\nKeep: the type\nNext: a hero of its own")
	if err != nil || r.Score != 6 || r.Passed || len(r.Issues) != 2 || r.Issues[0] != "Redo: the hero is a template split" || r.Keep != "the type" {
		t.Fatalf("%+v %v", r, err)
	}
	r.Round = 1
	if told := Told(r, "--pass done"); !strings.Contains(told, "6/10, under the bar of 8") || !strings.Contains(told, "then --pass done") {
		t.Fatalf("told: %s", told)
	}
	r.Round = MaxRounds
	if told := Told(r, "--pass done"); !strings.Contains(told, "last round") {
		t.Fatalf("last: %s", told)
	}
	if _, err := Parse("Looks good"); err == nil {
		t.Fatal("a verdict with no score was taken")
	}
	if r, _ := Parse("Score: 8"); !r.Passed || !strings.Contains(Told(proto.DesignReview{Round: 1, Score: 8, Passed: true}, ""), "it passes") {
		t.Fatal("8 passes")
	}
}
