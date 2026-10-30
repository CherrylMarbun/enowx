package daemon

import (
	"encoding/json"
	"strings"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// hasPhoneDesign says canvas cv has a design of its own at a phone's
// width: its wide designs aren't meant to reflow to one.
func hasPhoneDesign(cv proto.Canvas) bool {
	for _, f := range cv.Frames {
		if f.Width > 0 && f.Width <= 500 && strings.TrimSpace(f.HTML) != "" {
			return true
		}
	}
	return false
}

// A canvas's design measured as rendered, by the app: its lint at its own
// width (contrast, body text, kickers, stripes, cards in cards…) and, a
// wide design, its layout at a phone's. None when no app renders.
func (s *Server) canvasRendered(cv proto.Canvas, f proto.CanvasFrame) []proto.UnslopFinding {
	assets := func(name string) ([]byte, error) { return s.canvasFor(cv.ID).Asset(cv.ID, name) }
	run := func(g proto.CanvasFrame, script string) (string, bool) {
		r, err := s.appRender(proto.CanvasRenderRequest{HTML: canvas.FrameDocument(g, assets), Width: g.Width, Height: g.Height, Script: script})
		return r.Text, err == nil && r.Text != ""
	}
	text, ok := run(f, unslop.LintScript)
	if !ok {
		return nil
	}
	found := unslop.LintFindings(text, f.Width)
	if f.Width > 600 && !hasPhoneDesign(cv) {
		g := f
		g.Width, g.Height = 390, max(844, f.Height*390/max(f.Width, 1))
		if text, ok := run(g, layoutScript); ok {
			var l unslop.Layout
			// A page laid out at no width (a web view not on screen) says
			// nothing true of a phone's.
			if json.Unmarshal([]byte(text), &l) == nil && l.Width > 0 {
				found = append(found, unslop.LayoutFindings(l)...)
			}
		}
	}
	return found
}
