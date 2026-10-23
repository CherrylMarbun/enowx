package agent

import "testing"

func TestClaudeSettings(t *testing.T) {
	s := SettingsOf("claude")
	if c, ok := FindChoice(s.Models, "claude-opus-5-5"); !ok || c.Label != "Opus 5.5" {
		t.Fatalf("opus 5.5 = %+v, %v", c, ok)
	}
	if c, ok := FindChoice(s.Models, "claude-opus-5-5[1m]"); !ok || c.Label != "Opus 5.5" {
		t.Fatalf("opus 5.5 1m = %+v, %v", c, ok)
	}
	for model, want := range map[string]int64{
		"claude-opus-5-5":       1_000_000,
		"claude-sonnet-5-5[1m]": 1_000_000,
		"claude-haiku-4-5":      0, // not checked: unknown
	} {
		if got := s.ContextWindow(model); got != want {
			t.Errorf("ContextWindow(%s) = %d, want %d", model, got, want)
		}
	}
	screen := "❯ Try \"refactor\"\n  ⏸ plan mode on (shift+tab to cycle) · ← 2 agents\n"
	if got := s.ModeShown(screen); got != "plan" {
		t.Fatalf("ModeShown = %q", got)
	}
}
