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
	// A list whose selected option, marked as agents mark it, says yes.
	trustYes = regexp.MustCompile(`^\s*[>❯›▶]\s*(\d+\.\s*)?(yes|trust)\b`)
)

// TrustAsked reports whether the agent's screen asks to trust the folder,
// with an option that says yes selected: Enter answers it.
func TrustAsked(screen string) bool {
	asked := false
	for line := range strings.SplitSeq(screen, "\n") {
		line = strings.Trim(line, " │|")
		if trustQuestion.MatchString(line) {
			asked = true
			continue
		}
		if asked && trustYes.MatchString(strings.ToLower(line)) {
			return true
		}
	}
	return false
}
