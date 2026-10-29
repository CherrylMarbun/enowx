package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/enowx/enowx/core/unslop"
	"github.com/enowx/enowx/proto"
)

const unslopUsage = `unslop CANVAS [--frame F] [--start | --pass N|done [--region R] | --fanout "#a,#b" | --wait]
                                   what makes a mockup's design look AI-made, by pass, each with its fix and a score;
                                   --start copies the design and prints what to redesign on the copy;
                                   --pass says the pass of that redesign you begin (the app shows it), --region as a region's agent;
                                   --fanout starts an agent on each region for the later passes; --wait waits for them`

// unslopWait is the longest --wait waits for the regions' agents.
const unslopWait = 45 * time.Minute

func (r runner) canvasUnslop(args []string) error {
	fs := flag.NewFlagSet("unslop", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	frame := fs.String("frame", "", "")
	start := fs.Bool("start", false, "")
	pass := fs.String("pass", "", "")
	region := fs.String("region", "", "")
	fanout := fs.String("fanout", "", "")
	wait := fs.Bool("wait", false, "")
	pos, err := parseAll(fs, args)
	if err != nil || len(pos) != 1 {
		return errors.New("usage: enowx-app canvas " + unslopUsage)
	}
	c, err := r.findCanvas(pos[0])
	if err != nil {
		return err
	}
	if *wait {
		return r.unslopWait(c.ID)
	}
	p := proto.CanvasUnslopParams{ID: c.ID, Frame: *frame, Start: *start, Pass: *pass, Region: *region, Session: os.Getenv("ENOWX_SESSION")}
	p.Regions = strings.FieldsFunc(*fanout, func(r rune) bool { return r == ',' || r == ' ' })
	u, err := r.c.CanvasUnslop(p)
	if err != nil {
		return err
	}
	fmt.Fprint(r.out, unslop.Report(u.Score, u.Findings))
	if u.Ask != "" {
		fmt.Fprintf(r.out, "\n%s\n", u.Ask)
	}
	if len(p.Regions) > 0 && u.Run != nil {
		fmt.Fprintf(r.out, "\nAgents started on %d regions; wait for them: enowx-app canvas unslop %s --wait\n", len(u.Run.Regions), c.ID)
	}
	return nil
}

// unslopWait waits for the regions' agents of canvas id's unslop, saying
// each one's progress as it changes.
func (r runner) unslopWait(id string) error {
	said := map[string]string{}
	deadline := time.Now().Add(unslopWait)
	for {
		u, err := r.c.CanvasUnslop(proto.CanvasUnslopParams{ID: id, Status: true})
		if err != nil {
			return err
		}
		run := u.Run
		left := 0
		for _, reg := range run.Regions {
			state := reg.Pass
			if reg.Done {
				state = "done"
			} else {
				left++
			}
			if state == "" {
				state = "starting"
			}
			if said[reg.ID] != state {
				said[reg.ID] = state
				fmt.Fprintf(r.out, "#%s: %s\n", reg.ID, state)
			}
		}
		if left == 0 {
			fmt.Fprintf(r.out, "All regions done; score %d/100. Now check the whole design (access, mobile), then --pass done.\n", run.Score)
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("canvas: %d regions still at work after %s", left, unslopWait)
		}
		time.Sleep(3 * time.Second)
	}
}
