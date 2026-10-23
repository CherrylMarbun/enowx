package agent

import (
	"regexp"
	"strings"
)

// Many agents ask whether to trust the folder they start in before they
// take a prompt. The folder is the one the user picked in enowX for the
// session, which is that decision made: enowX answers yes for them, so a
// first prompt doesn't wait behind the question.

var (
	trustQuestion = regexp.MustCompile(`(?i)\btrust\b.*\?`)
	// The selected option of a list, marked as agents mark it.
	selected = regexp.MustCompile(`^\s*[>❯›▶]\s*\S`)
	// An option that says yes, selected or not.
	trustYes = regexp.MustCompile(`^\s*([>❯›▶]\s*)?(\d+\.\s*)?(yes|trust)\b`)
)

// TrustAsked reports whether the agent's screen asks to trust the folder,
// with an option that says yes (TrustMoves).
func TrustAsked(screen string) bool {
	_, ok := TrustMoves(screen)
	return ok
}

// TrustMoves reads a question about trusting the folder: how many lines
// the selection moves to reach the option saying yes (down when positive,
// 0 when it is selected), after which Enter answers. Some agents select
// "No" first.
func TrustMoves(screen string) (int, bool) {
	asked := false
	sel, yes, n := -1, -1, 0 // indexes among the non-blank lines after the question
	for line := range strings.SplitSeq(screen, "\n") {
		line = strings.ToLower(strings.Trim(line, " │|"))
		if !asked {
			asked = trustQuestion.MatchString(line)
			continue
		}
		if line == "" {
			continue
		}
		if selected.MatchString(line) && sel < 0 {
			sel = n
		}
		if trustYes.MatchString(line) && yes < 0 {
			yes = n
		}
		n++
	}
	if sel < 0 || yes < 0 {
		return 0, false
	}
	return yes - sel, true
}
