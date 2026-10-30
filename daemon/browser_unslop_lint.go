package daemon

import (
	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// lintNow lints the page in session's Browser as rendered now, width CSS
// pixels wide: what its computed styles show (contrast, body text,
// entrances never run, kickers, stripes, cards in cards…). LintScript
// hides the app's own marks while it measures. None when the app can't
// run it (an older app, a page gone).
func (s *Server) lintNow(session string, width int) []proto.UnslopFinding {
	res, err := s.browserAct(proto.BrowserAction{Session: session, Action: proto.BrowserEval, Text: unslop.LintScript})
	if err != nil {
		return nil
	}
	return unslop.LintFindings(res.Text, width)
}
