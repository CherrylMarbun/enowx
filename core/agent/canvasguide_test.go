package agent

import (
	"regexp"
	"strings"
	"testing"
)

// Every library the canvas guide loads from a CDN names an exact version,
// so a mockup made today looks the same tomorrow.
func TestCanvasGuidePinsLibraries(t *testing.T) {
	urls := regexp.MustCompile(`https://(cdn\.jsdelivr\.net|cdnjs\.cloudflare\.com|cdn\.tailwindcss\.com|unpkg\.com)[^\s"'<>)]*`).FindAllString(CanvasGuide, -1)
	if len(urls) < 8 {
		t.Fatalf("only %d CDN links: %q", len(urls), urls)
	}
	pinned := regexp.MustCompile(`(@|/)\d+\.\d+\.\d+(/|$)`)
	for _, u := range urls {
		if !pinned.MatchString(u) || strings.Contains(u, "latest") {
			t.Errorf("%s names no exact version", u)
		}
	}
	for _, want := range []string{"canvas grid ID", "canvas icon lucide:rocket", "Offline", "prefers-reduced-motion"} {
		if !strings.Contains(CanvasGuide, want) {
			t.Errorf("the guide doesn't say %q", want)
		}
	}
}
