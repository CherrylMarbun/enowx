package daemon

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

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
	if p.Status {
		run, ok := s.unslopRuns.status(cv.ID)
		if !ok {
			return proto.CanvasUnslop{}, errors.New("canvas: no unslop under way on this canvas")
		}
		return proto.CanvasUnslop{Frame: run.Frame, Score: run.Score, Findings: run.Findings, Run: &run}, nil
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
	theme := unslop.Theme(from.HTML)
	if !p.Start {
		theme = s.unslopRuns.theme(cv.ID, f.ID)
	}
	// A redesign keeps its original's theme.
	if fd, turned := unslop.ThemeFinding(theme, f.HTML); turned {
		findings = append([]proto.UnslopFinding{fd}, findings...)
		score = max(score-15, 0)
	}
	out := proto.CanvasUnslop{Frame: f.ID, Score: score, Findings: findings}
	if p.Start {
		out.Ask = unslopAsk(cv, i+1, n, f, score, len(findings), theme)
	}
	switch {
	case p.Region != "":
		// A region's agent: its pass, the run's score kept current.
		if _, _, err := s.unslopRuns.update(cv.ID, f.ID, proto.CanvasUnslopParams{}, score, findings, theme); err != nil {
			return out, err
		}
		run, err := s.unslopRuns.regionReport(cv.ID, f.ID, p.Region, p.Pass)
		if err != nil {
			return out, err
		}
		s.broadcast(proto.EventCanvasUnslop, run)
		out.Run = &run
		return out, nil
	case len(p.Regions) > 0:
		if err := s.fanOutRegions(cv, f, n, p.Regions, p.Session); err != nil {
			return out, err
		}
	}
	run, ok, err := s.unslopRuns.update(cv.ID, f.ID, proto.CanvasUnslopParams{Start: p.Start, Pass: p.Pass}, score, findings, theme)
	if err != nil {
		return out, err
	}
	if ok {
		s.broadcast(proto.EventCanvasUnslop, run)
		out.Run = &run
	}
	return out, nil
}

// unslopRuns are the canvases' redesigns under way, by canvas: one each,
// the latest started. mu guards runs.
type unslopRuns struct {
	mu   sync.Mutex
	runs map[string]*proto.CanvasUnslopRun
}

// update records a read of design frame of canvas id: a run started, or a
// pass begun, or the design read again in one under way; false when no
// run is about it.
func (u *unslopRuns) update(id, frame string, p proto.CanvasUnslopParams, score int, findings []proto.UnslopFinding, theme string) (proto.CanvasUnslopRun, bool, error) {
	pass, done := "", strings.EqualFold(strings.TrimSpace(p.Pass), "done")
	if p.Pass != "" && !done {
		name, ok := unslop.PassNamed(p.Pass)
		if !ok {
			return proto.CanvasUnslopRun{}, false, fmt.Errorf("canvas: no pass %q: a number from 1 to %d, or done", p.Pass, len(unslop.Passes))
		}
		pass = name
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.runs == nil {
		u.runs = map[string]*proto.CanvasUnslopRun{}
	}
	run := u.runs[id]
	switch {
	case p.Start:
		run = &proto.CanvasUnslopRun{ID: id, Frame: frame, From: score, Passes: unslop.Names(), Theme: theme}
		u.runs[id] = run
	case run == nil || run.Frame != frame:
		return proto.CanvasUnslopRun{}, false, nil
	}
	run.Score, run.Findings = score, findings
	run.Scans++
	if (pass != "" || done) && run.Pass != "" && run.Pass != pass && !slices.Contains(run.Finished, run.Pass) {
		run.Finished = append(run.Finished, run.Pass)
	}
	switch {
	case done:
		run.Pass, run.Done = "", true
	case pass != "":
		run.Pass = pass
	}
	return copyRun(run), true, nil
}

// theme is the original's theme of the run about design frame of canvas
// id, "" when none.
func (u *unslopRuns) theme(id, frame string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if run := u.runs[id]; run != nil && run.Frame == frame {
		return run.Theme
	}
	return ""
}

// unslopAsk is what the canvas's agent is told to redesign design n, the
// copy of design from: the command telling it what's off, by pass, and
// how to go through them.
func unslopAsk(cv proto.Canvas, from, n int, f proto.CanvasFrame, score, found int, theme string) string {
	keep := "Keep its look's direction: its theme, brand color family and mood stay; unslop changes its care, not its style."
	if theme != "" {
		keep = "It is a " + theme + " design and stays " + theme + ": never turn its theme. " + keep
	}
	cmd := fmt.Sprintf(`"$ENOWX_BIN" canvas unslop %s --frame %s`, cv.ID, f.ID)
	return fmt.Sprintf(`Unslop design %d (“%s”), a copy of design %d: it scores %d/100 with %d findings of what makes a design look AI-made. The user watches it live.
1. Run: %s — it prints the findings by pass, in the order to fix them, each with its fix.
2. Redesign design %d one pass at a time, in that order, keeping its content, purpose and layout's intent; never invent data. %s Passes with nothing found still get a designer's look (structure, mobile).
3. Begin each pass with: %s --pass N (N its number, 1 to 12; it reads the design again, and the app shows the pass under way). Change the design element by element (canvas patch), not all at once, so each change shows.
4. Do passes 1 to 5 yourself (truth, structure, spacing, type, color): they set the foundation, with the spacing, type and color as CSS variables the rest uses.
5. Then fan passes 6 to 10 out, an agent per region at once: give each element with an id that is a part of its own (2 to 6 of them: the sidebar, the header, a row of cards, the chart…, none inside another, together most of the design) to: %s --fanout "#a,#b,#c". Then wait for them: "$ENOWX_BIN" canvas unslop %s --wait. Don't change the design meanwhile. A design too small to split, do 6 to 10 yourself.
6. Then do 11 (access) and 12 (mobile) on the whole design; after each pass, describe the design with what it changed ("$ENOWX_BIN" canvas design %s %d --description "…").
7. When what it still finds is what you'd keep on purpose: %s --pass done, then show both: "$ENOWX_BIN" canvas compare %s %d %d.`,
		n, f.Name, from, score, found, cmd, n, keep, cmd, cmd, cv.ID, cv.ID, n, cmd, cv.ID, from, n)
}
