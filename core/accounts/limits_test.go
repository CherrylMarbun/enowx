package accounts

import (
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// What agents print at a limit reads as one, with when it resets.
func TestClassify(t *testing.T) {
	now := time.Date(2026, 10, 31, 10, 20, 0, 0, time.Local)
	cases := []struct {
		screen, reason string
		until          time.Time
	}{
		{"⏺ Working…\n  ⎿  Claude usage limit reached. Your limit will reset at 3pm (Asia/Jakarta).\n> ", proto.AccountLimit, time.Date(2026, 10, 31, 15, 0, 0, 0, time.Local)},
		{"■ You've hit your usage limit. Upgrade to Pro or try again in 2 hours 15 minutes.", proto.AccountLimit, now.Add(2*time.Hour + 15*time.Minute)},
		{"5-hour limit reached ∙ resets 9am", proto.AccountLimit, time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)},
		{"API Error: 400 Your credit balance is too low to access the Anthropic API.", proto.AccountCredits, time.Time{}},
		{"Invalid API key · Please run /login", proto.AccountAuth, time.Time{}},
		{"stream error: exceeded retry limit, last status: 429 Too Many Requests", proto.AccountLimit, time.Time{}},
	}
	for _, c := range cases {
		l, ok := Classify(c.screen, now)
		if !ok || l.Reason != c.reason || !l.Until.Equal(c.until) {
			t.Errorf("%q: %+v %v", c.screen, l, ok)
		}
	}
	if _, ok := Classify("All tests pass. The rate of change is fine.", now); ok {
		t.Error("a plain line read as a limit")
	}
}

// The next account by priority, skipping those spent or kept to other
// projects; defaults by project.
func TestNextAndDefault(t *testing.T) {
	pr := proto.AccountPrefs{Order: []string{"Work", "", "Spare"}, Only: map[string][]string{"Work": {"acme"}}, Projects: map[string]string{"site": "Spare"}}
	names := []string{"Work", "Spare"}
	if got := Ordered(pr, names); len(got) != 3 || got[0] != "Work" || got[1] != "" {
		t.Fatalf("ordered %q", got)
	}
	spent := func(n string) bool { return n == "" }
	if n, ok := Next(pr, names, "Work", "/code/acme", spent); !ok || n != "Spare" {
		t.Fatalf("next %q %v", n, ok)
	}
	if n, ok := Next(pr, names, "Spare", "/code/other", spent); ok {
		t.Fatalf("Work isn't for other projects, the own is spent: %q", n)
	}
	if Default(pr, "/code/site") != "Spare" || Default(pr, "/code/x") != "" {
		t.Fatal("defaults")
	}
}

// A key in the line an agent printed isn't kept.
func TestLimitLineMasksKeys(t *testing.T) {
	l, ok := Classify("Error: Invalid API key sk-ant-api03-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789", time.Now())
	if !ok || strings.Contains(l.Line, "AbCdEfGh") || !strings.Contains(l.Line, "REDACTED") {
		t.Fatalf("line %q", l.Line)
	}
}
