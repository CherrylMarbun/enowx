package cli

import (
	"bytes"
	"strings"
	"testing"
)

// An agent finds an icon for a note by a word, without the daemon.
func TestIconsSearch(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"icons", "search", "rocket", "--lib", "lucide", "--limit", "3"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "lucide:rocket ") {
		t.Fatalf("search:\n%s", out.String())
	}
	out.Reset()
	if err := Run([]string{"icons", "search", "arrow", "--limit", "3"}, &out); err != nil || strings.Count(out.String(), "\n") != 3 {
		t.Fatalf("limit: %v\n%s", err, out.String())
	}
	if err := Run([]string{"icons", "search", "zzzqqq"}, &out); err == nil {
		t.Error("no icon, no error")
	}
}
