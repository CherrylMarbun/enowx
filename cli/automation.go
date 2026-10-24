package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/enowx/enowx/proto"
)

// automation runs `enowx automation add|run|enable|disable|remove`.
func (r runner) automation(args []string) error {
	if len(args) == 0 {
		return r.automations()
	}
	switch args[0] {
	case "add":
		return r.addAutomation(args[1:])
	case "run", "enable", "disable", "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: enowx-app automation %s ID", args[0])
		}
		a, err := r.findAutomation(args[1])
		if err != nil {
			return err
		}
		switch args[0] {
		case "run":
			s, err := r.c.RunAutomation(a.ID)
			if err == nil {
				fmt.Fprintln(r.out, s.ID)
			}
			return err
		case "remove":
			return r.c.RemoveAutomation(a.ID)
		}
		a.Enabled = args[0] == "enable"
		_, err = r.c.SaveAutomation(a)
		return err
	}
	return errors.New("usage: enowx-app automation [add|run|enable|disable|remove] …")
}

func (r runner) addAutomation(args []string) error {
	fs := flag.NewFlagSet("automation add", flag.ContinueOnError)
	name := fs.String("name", "", "")
	schedule := fs.String("schedule", "", "")
	agentID := fs.String("agent", "claude", "")
	projectDir := fs.String("project", "", "")
	branch := fs.Bool("branch", false, "")
	yolo := fs.Bool("yolo", false, "")
	disabled := fs.Bool("disabled", false, "")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil || fs.NArg() == 0 || *name == "" || *schedule == "" {
		return errors.New(`usage: enowx-app automation add --name NAME --schedule "weekdays 09:00" [--agent ID] [--project DIR] [--branch] [--yolo] [--disabled] PROMPT`)
	}
	dir := *projectDir
	if dir == "" {
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
	a, err := r.c.SaveAutomation(proto.Automation{
		Name: *name, Schedule: *schedule, Agent: *agentID, Project: abs, Prompt: strings.Join(fs.Args(), " "),
		Branch: *branch, SkipPermissions: *yolo, Enabled: !*disabled,
	})
	if err != nil {
		return err
	}
	fmt.Fprintln(r.out, a.ID)
	return nil
}

// findAutomation finds an automation by its ID, the start of one, or its
// name.
func (r runner) findAutomation(ref string) (proto.Automation, error) {
	list, err := r.c.Automations()
	if err != nil {
		return proto.Automation{}, err
	}
	var found []proto.Automation
	for _, a := range list {
		if a.ID == ref {
			return a, nil
		}
		if strings.HasPrefix(a.ID, ref) || strings.EqualFold(a.Name, ref) {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		return proto.Automation{}, fmt.Errorf("%q names %d automations", ref, len(found))
	}
	return found[0], nil
}

func (r runner) automations() error {
	list, err := r.c.Automations()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSCHEDULE\tAGENT\tPROJECT\tNEXT\tLAST")
	for _, a := range list {
		next := "off"
		if a.Enabled && !a.NextRun.IsZero() {
			next = a.NextRun.Format("Mon Jan 2 15:04")
		}
		last := "-"
		switch {
		case a.LastError != "":
			last = "failed: " + a.LastError
		case !a.LastRun.IsZero():
			last = a.LastRun.Format("Jan 2 15:04") + " " + a.LastSession
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", a.ID, a.Name, a.Schedule, a.Agent, filepath.Base(a.Project), next, last)
	}
	return w.Flush()
}
