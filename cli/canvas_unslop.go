package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

const unslopUsage = `unslop CANVAS [--frame F] [--start]
                                   what makes a mockup's design look AI-made, by pass, each with its fix and a score;
                                   --start copies the design and prints what to redesign on the copy`

func (r runner) canvasUnslop(args []string) error {
	fs := flag.NewFlagSet("unslop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	frame := fs.String("frame", "", "")
	start := fs.Bool("start", false, "")
	pos, err := parseAll(fs, args)
	if err != nil || len(pos) != 1 {
		return errors.New("usage: enowx-app canvas " + unslopUsage)
	}
	c, err := r.findCanvas(pos[0])
	if err != nil {
		return err
	}
	u, err := r.c.CanvasUnslop(proto.CanvasUnslopParams{ID: c.ID, Frame: *frame, Start: *start})
	if err != nil {
		return err
	}
	fmt.Fprint(r.out, unslop.Report(u.Score, u.Findings))
	if u.Ask != "" {
		fmt.Fprintf(r.out, "\n%s\n", u.Ask)
	}
	return nil
}
