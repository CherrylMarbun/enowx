package agent

import "testing"

func TestTrustAsked(t *testing.T) {
	cases := map[string]bool{
		// CodeBuddy Code, in a box.
		"│ Do you trust the files in this folder?   │\n│                │\n│   > 1. Trust folder only (005)      │\n│     4. No, exit (escape)  │": true,
		// Antigravity CLI.
		"Do you trust the contents of this project?\nAntigravity CLI requires permission.\n> Yes, I trust this folder\n  No, exit": true,
		// The selection on No: not answered.
		"Do you trust the contents of this project?\n  Yes, I trust this folder\n> No, exit": false,
		// A question without a list, or an answer about trust elsewhere.
		"Do you trust me?": false,
		"> Yes": false,
	}
	for screen, want := range cases {
		if got := TrustAsked(screen); got != want {
			t.Errorf("TrustAsked(%q) = %v, want %v", screen, got, want)
		}
	}
}
