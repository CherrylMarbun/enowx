package daemon

import (
	"encoding/json"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

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
	if f.Width > 600 {
		g := f
		g.Width, g.Height = 390, max(844, f.Height*390/max(f.Width, 1))
		if text, ok := run(g, layoutScript); ok {
			var l unslop.Layout
			if json.Unmarshal([]byte(text), &l) == nil {
				found = append(found, unslop.LayoutFindings(l)...)
			}
		}
	}
	return found
}
