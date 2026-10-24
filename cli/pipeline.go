package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

var notBranchChar = regexp.MustCompile(`[^a-z0-9]+`)

func (r runner) pipeline(args []string) error {
	fs := flag.NewFlagSet("pipeline", flag.ContinueOnError)
	branch := fs.String("branch", "", "")
	agents := fs.String("agents", "claude,claude,claude", "")
	projectDir := fs.String("project", "", "")
	yolo := fs.Bool("yolo", false, "")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 {
		return errors.New("usage: enowx-app pipeline [--branch NAME] [--agents A,B,C] [--project DIR] [--yolo] TASK")
	}
	task := strings.Join(fs.Args(), " ")
	if *branch == "" {
		words := strings.Fields(strings.ToLower(task))
		*branch = "pipeline/" + strings.Trim(notBranchChar.ReplaceAllString(strings.Join(words[:min(len(words), 5)], "-"), "-"), "-")
	}
	ids := strings.Split(*agents, ",")
	stages := proto.DefaultPipeline("claude")
	if len(ids) != len(stages) {
		return fmt.Errorf("pipeline: --agents names %d agents for %d stages: plan, implement, review", len(ids), len(stages))
	}
	for i := range stages {
		stages[i].Agent = strings.TrimSpace(ids[i])
	}
	dir := *projectDir
	switch {
	case dir != "":
	case os.Getenv(EnvSession) != "":
		self, err := r.session("self")
		if err != nil {
			return err
		}
		dir = self.Project
	default:
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if _, err := r.c.AddProject(abs); err != nil {
		return err
	}
	pl, err := r.c.StartPipeline(proto.PipelineStartParams{Project: abs, Task: task, Branch: *branch, Stages: stages, SkipPermissions: *yolo})
	if err != nil {
		return err
	}
	fmt.Fprintln(r.out, pl.ID)
	return nil
}

func (r runner) pipelines() error {
	list, err := r.c.Pipelines()
	if err != nil {
		return err
	}
	for _, p := range list {
		var stages []string
		for _, st := range p.Stages {
			s := st.Name + ":" + st.Status
			if st.Session != "" {
				s += "(" + st.Session + ")"
			}
			stages = append(stages, s)
		}
		fmt.Fprintf(r.out, "%s  %s  %s  %s\n  %s\n", p.ID, p.Status, p.Branch, filepath.Base(p.Project), strings.Join(stages, " → "))
	}
	return nil
}
