package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/enowx/enowx/core/designcompare"
	"github.com/enowx/enowx/proto"
)

const buildUsage = `build CANVAS --start --what TEXT [--directions N]   start a UI build (as the app's Build does) and print what to tell its agent
  build CANVAS [--stage S | --fanout N | --done [--frame F]]   the UI build under way: say its stage
                                   (brief, directions, building, compare, refining), start N fresh agents on a direction each,
                                   or say it is done (a direction's agent: its design; the lead: --frame, the design to review)
  direction CANVAS --frame F [--card TEXT|-]   a design's direction card (North star:, Source:, Skeleton:, Type:, Palette:…)
  match CANVAS --frame F --image NAME          how far a design is from the picture it recreates`

// buildTools are the UI builder's tools for the canvas's agents.
var buildTools = []mcpTool{imageTools,
	{name: "canvas_build", description: "The UI build under way on a canvas: say its stage as you go, start fresh agents on a direction each (fanout), or say done: a direction's agent when its design is built; the lead with the design to review independently.",
		props: map[string]mcpProp{"canvas": canvasProp, "stage": {"string", "brief, directions, building, compare or refining"},
			"fanout": {"number", "As the lead: start this many fresh agents (2 to 4), a direction each"},
			"done":   {"boolean", "Done: your direction (as its agent), or the build (as the lead, with design)"}, "design": designProp}, required: []string{"canvas"},
		args: func(a map[string]any) []string {
			args := withFlag(withFlag([]string{"build", str(a, "canvas")}, a, "stage", "stage"), a, "fanout", "fanout")
			if b, _ := a["done"].(bool); b {
				args = append(args, "--done")
			}
			return withFlag(args, a, "design", "frame")
		}},
	{name: "canvas_direction", description: "A design's direction card: without card it prints it; with card (North star:, Source:, Skeleton:, Type:, Palette:, Hero:, Icons:, Controls:, Moment:, Gives up:, a line each) it keeps it. Write it before building the direction.",
		props: map[string]mcpProp{"canvas": canvasProp, "design": designProp, "card": {"string", "The card, a \"Field: value\" line each"}}, required: []string{"canvas", "design"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag([]string{"direction", str(a, "canvas")}, a, "design", "frame"), a, "card", "card")
		}},
	{name: "canvas_match", description: "How far a design is from the picture it recreates (a canvas asset): the share of pixels that differ, overall and in a 3×3 grid, the worst cells named. Run it after each round of a recreation.",
		props: map[string]mcpProp{"canvas": canvasProp, "design": designProp, "image": {"string", "The picture's asset name"}}, required: []string{"canvas", "design", "image"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag([]string{"match", str(a, "canvas")}, a, "design", "frame"), a, "image", "image")
		}},
}

func (r runner) canvasBuild(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	stage := fs.String("stage", "", "")
	fanout := fs.Int("fanout", 0, "")
	done := fs.Bool("done", false, "")
	frame := fs.String("frame", "", "")
	card := fs.String("card", "", "")
	image := fs.String("image", "", "")
	start := fs.Bool("start", false, "")
	what := fs.String("what", "", "")
	directions := fs.Int("directions", 1, "")
	pos, err := parseAll(fs, args)
	if err != nil || len(pos) != 1 {
		return errors.New("usage: enowx-app canvas " + buildUsage)
	}
	c, err := r.findCanvas(pos[0])
	if err != nil {
		return err
	}
	session := os.Getenv("ENOWX_SESSION")
	switch cmd {
	case "direction":
		return r.canvasDirection(c, *frame, *card, session)
	case "match":
		m, err := r.c.CanvasMatch(proto.CanvasMatchParams{ID: c.ID, Frame: *frame, Image: *image})
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "%.1f%% of pixels differ.\n", m.Diff)
		names := []string{"top left", "top", "top right", "left", "middle", "right", "bottom left", "bottom", "bottom right"}
		for i, v := range m.Cells {
			fmt.Fprintf(r.out, "  %-13s %5.1f%%\n", names[i], v)
		}
		if len(m.Worst) > 0 {
			fmt.Fprintf(r.out, "Fix first: %s.\n", strings.Join(m.Worst, ", "))
		}
		return nil
	}
	if *start {
		b, err := r.c.CanvasBuild(proto.CanvasBuildParams{ID: c.ID, Start: true, What: *what, Directions: *directions})
		if err != nil {
			return err
		}
		fmt.Fprintln(r.out, b.Ask)
		return nil
	}
	b, err := r.c.CanvasBuild(proto.CanvasBuildParams{ID: c.ID, Stage: *stage, Fanout: *fanout, Done: *done, Frame: *frame, Session: session})
	if err != nil {
		return err
	}
	run := b.Run
	switch {
	case *fanout > 0:
		fmt.Fprintf(r.out, "%d agents are building a direction each. Change nothing and end your turn: you're told when they're built.\n", *fanout)
	case *done && run != nil && run.Stage == proto.BuildReviewing:
		fmt.Fprintln(r.out, "A fresh agent reviews it now; you're told its verdict. End your turn.")
	case *done:
		fmt.Fprintln(r.out, "Done. You're told if it must change.")
	case run != nil:
		fmt.Fprintf(r.out, "Stage: %s.\n", run.Stage)
	}
	return nil
}

// canvasDirection prints design frame's card, or keeps card.
func (r runner) canvasDirection(c proto.Canvas, frame, card, session string) error {
	if card == "" {
		for i, f := range c.Frames {
			if f.ID == frame || fmt.Sprint(i+1) == frame || f.Name == frame {
				if f.Direction == nil {
					fmt.Fprintf(r.out, "No card yet. Write one, a line each:\n%s\n", designcompare.CardFormat)
					return nil
				}
				fmt.Fprint(r.out, designcompare.CardText(*f.Direction))
				return nil
			}
		}
		return fmt.Errorf("canvas: no design %q", frame)
	}
	text, err := stdinOr(card)
	if err != nil {
		return err
	}
	d, err := designcompare.ParseCard(text)
	if err != nil {
		return fmt.Errorf("canvas: %w; write it in this form:\n%s", err, designcompare.CardFormat)
	}
	if _, err := r.c.EditCanvas(proto.CanvasEdit{ID: c.ID, Op: proto.CanvasOpDirection, Frame: frame, Direction: &d, Session: session}); err != nil {
		return err
	}
	fmt.Fprintln(r.out, "Card kept; the app shows it with the design.")
	return nil
}
