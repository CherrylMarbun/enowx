package daemon

import (
	"errors"
	"fmt"

	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

// canvasUnslop reads a mockup's design for what makes it look AI-made
// (core/unslop); started, it copies the design first, so the redesign
// happens on the copy and the original stays to compare, and says what to
// ask the canvas's agent.
func (s *Server) canvasUnslop(p proto.CanvasUnslopParams) (proto.CanvasUnslop, error) {
	cv, err := s.canvasFor(p.ID).Get(p.ID)
	if err != nil {
		return proto.CanvasUnslop{}, err
	}
	i, err := canvas.FrameIndex(cv, p.Frame)
	if err != nil {
		return proto.CanvasUnslop{}, err
	}
	from := cv.Frames[i]
	if from.HTML == "" {
		return proto.CanvasUnslop{}, errors.New("canvas: unslop reads a mockup's HTML, and this design has none")
	}
	f, n := from, i+1
	if p.Start {
		name := from.Name + " · unslop"
		desc := "Unslopped from design " + fmt.Sprint(i+1)
		c, err := s.canvasEdit(proto.CanvasEdit{ID: cv.ID, Op: proto.CanvasOpFrameCopy, Frame: from.ID, Name: &name, Description: &desc})
		if err != nil {
			return proto.CanvasUnslop{}, err
		}
		cv = c
		if i+1 >= len(cv.Frames) {
			return proto.CanvasUnslop{}, errors.New("canvas: the design's copy isn't there")
		}
		f, n = cv.Frames[i+1], i+2
	}
	score, findings := unslop.Scan(f.HTML, cv.Kind)
	out := proto.CanvasUnslop{Frame: f.ID, Score: score, Findings: findings}
	if p.Start {
		out.Ask = unslopAsk(cv, i+1, n, f, score, len(findings))
	}
	return out, nil
}

// unslopAsk is what the canvas's agent is told to redesign design n, the
// copy of design from: the command telling it what's off, by pass, and
// how to go through them.
func unslopAsk(cv proto.Canvas, from, n int, f proto.CanvasFrame, score, found int) string {
	return fmt.Sprintf(`Unslop design %d (“%s”), a copy of design %d: it scores %d/100 with %d findings of what makes a design look AI-made.
1. Run: "$ENOWX_BIN" canvas unslop %s --frame %s — it prints the findings by pass, in the order to fix them, each with its fix.
2. Redesign design %d one pass at a time, in that order, keeping its content, purpose and layout's intent; never invent data. Passes with nothing found still get a designer's look (structure, mobile).
3. After each pass, describe the design with what that pass changed ("$ENOWX_BIN" canvas design %s %d --description "…"), then run the unslop command again.
4. Stop when what it still finds is what you'd keep on purpose; then show both: "$ENOWX_BIN" canvas compare %s %d %d.`,
		n, f.Name, from, score, found, cv.ID, f.ID, n, cv.ID, n, cv.ID, from, n)
}
