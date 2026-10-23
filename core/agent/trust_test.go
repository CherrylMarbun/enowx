package agent

import "testing"

func TestTrustAsked(t *testing.T) {
	cases := map[string]bool{
		// CodeBuddy Code, in a box.
		"│ Do you trust the files in this folder?   │\n│                │\n│   > 1. Trust folder only (005)      │\n│     4. No, exit (escape)  │": true,
		// Antigravity CLI.
		"Do you trust the contents of this project?\nAntigravity CLI requires permission.\n> Yes, I trust this folder\n  No, exit": true,
		// The selection on No, Yes above it: one up.
		"Do you trust the contents of this project?\n  Yes, I trust this folder\n> No, exit": true,
		// A question without a list, or an answer about trust elsewhere.
		"Do you trust me?": false,
		"> Yes":            false,
	}
	for screen, want := range cases {
		if got := TrustAsked(screen); got != want {
			t.Errorf("TrustAsked(%q) = %v, want %v", screen, got, want)
		}
	}
}

// Claude Code selects "No, exit" first, below a description: one down.
func TestTrustMovesToYes(t *testing.T) {
	screen := ` Accessing workspace:
 /Users/me/proj
 Quick safety check: Is this a project you created or one you trust? (Like your own code, a well-known open source
 project, or work from your team). If not, take a moment to review what's in this folder first.
 Claude Code'll be able to read, edit, and execute files here.
 Security guide
 ❯ No, exit
   Yes, I trust this folder
 Enter to confirm · Esc to cancel`
	if moves, ok := TrustMoves(screen); !ok || moves != 1 {
		t.Fatalf("TrustMoves = %d, %v; want 1, true", moves, ok)
	}
	if moves, _ := TrustMoves("Do you trust the contents of this project?\n  Yes, I trust this folder\n> No, exit"); moves != -1 {
		t.Fatalf("up: %d", moves)
	}
}
