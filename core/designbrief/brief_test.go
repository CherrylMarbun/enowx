package designbrief

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A brief as written by an agent reads back the same, sections in any
// case and order; what's required and missing is named.
func TestBriefRoundTrip(t *testing.T) {
	b := Parse("# Brief\n\n## tone\nCalm, dense.\nMono for numbers.\n\n## Product:\nA read-only API's docs.\n\n## Notes\nignored\n")
	if b.Product != "A read-only API's docs." || b.Tone != "Calm, dense.\nMono for numbers." {
		t.Fatalf("parsed: %+v", b)
	}
	if err := Check(b); err == nil || !strings.Contains(err.Error(), "Users, First screen") {
		t.Fatalf("check: %v", err)
	}
	b.Users, b.FirstScreen = "Developers wiring a client.", "The endpoints, searchable."
	b.World, b.Signature = "A terminal at night: phosphor on graphite.", "Keycaps for every shortcut."
	if Check(b) != nil || Parse(Markdown(b)) != b {
		t.Fatalf("round trip: %+v", Parse(Markdown(b)))
	}
	if Summary(b) != "A read-only API's docs. · Calm, dense." || Parse(Template()) == (proto.DesignBrief{}) {
		t.Fatalf("summary %q", Summary(b))
	}
}
