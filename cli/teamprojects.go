package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/enowx/enowx/proto"
)

const teamProjectUsage = `usage: enowx-app team projects [TEAM]
       enowx-app team project add TEAM LINK [--name N]   (a team's admins)
       enowx-app team project rm TEAM PROJECT            (a team's admins)
       enowx-app team project clone TEAM PROJECT [--dir D]`

// teamProjects lists the teams' projects, and where each is on the host.
func (r runner) teamProjects(args []string) error {
	team := ""
	if len(args) > 0 {
		t, err := r.findSpace(strings.Join(args, " "))
		if err != nil {
			return err
		}
		team = t
	}
	list, err := r.c.TeamProjects()
	if err != nil {
		return err
	}
	names := map[string]string{}
	if spaces, err := r.c.Spaces(); err == nil {
		for _, sp := range spaces {
			names[sp.Team] = sp.Name
		}
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TEAM\tPROJECT\tLINK\tHERE")
	shown := 0
	for _, p := range list {
		if team != "" && p.Team != team {
			continue
		}
		here := "not cloned: enowx-app team project clone " + quoteArg(names[p.Team]) + " " + quoteArg(p.Name)
		if len(p.Here) > 0 {
			here = p.Here[0].Path
			if p.Here[0].Session != "" {
				here += " (session " + p.Here[0].Session + ")"
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", names[p.Team], p.Name, p.Repo, here)
		shown++
	}
	if shown == 0 {
		fmt.Fprintln(r.out, "No team projects: a team's admin adds one with \"enowx-app team project add TEAM LINK\".")
		return nil
	}
	return w.Flush()
}

// teamProject adds, removes or clones a team's project.
func (r runner) teamProject(args []string) error {
	if len(args) < 3 {
		return errors.New(teamProjectUsage)
	}
	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "")
	dir := fs.String("dir", "", "")
	rest, err := parseInterspersed(fs, args[1:])
	if err != nil || len(rest) < 2 {
		return errors.New(teamProjectUsage)
	}
	team, err := r.findSpace(rest[0])
	if err != nil {
		return err
	}
	if team == "" {
		return errors.New(teamProjectUsage)
	}
	switch cmd {
	case "add":
		p, err := r.c.AddTeamProject(proto.TeamProjectAddParams{Team: team, Repo: rest[1], Name: *name})
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Added %s (%s): the team's members see it, and clone it where they haven't.\n", p.Name, p.Repo)
		return nil
	case "rm", "clone":
		p, err := r.findTeamProject(team, strings.Join(rest[1:], " "))
		if err != nil {
			return err
		}
		if cmd == "rm" {
			return r.c.RemoveTeamProject(team, p.ID)
		}
		if len(p.Here) > 0 {
			fmt.Fprintf(r.out, "%s is on this host already: %s\n", p.Name, p.Here[0].Path)
			return nil
		}
		if *dir == "" {
			if prefs, err := r.c.ClonePrefs(); err == nil && prefs.Dir != "" {
				*dir = prefs.Dir
			} else if err == nil && prefs.Default != "" {
				*dir = prefs.Default
			}
		}
		if *dir == "" {
			*dir, _ = os.Getwd()
		}
		st, err := r.c.CloneProject(proto.ProjectCloneParams{URL: p.Repo, Dir: *dir})
		if err != nil {
			return err
		}
		return r.followClone(st.ID)
	}
	return errors.New(teamProjectUsage)
}

// findTeamProject is team's project named or IDed spec.
func (r runner) findTeamProject(team, spec string) (proto.TeamProject, error) {
	list, err := r.c.TeamProjects()
	if err != nil {
		return proto.TeamProject{}, err
	}
	for _, p := range list {
		if p.Team == team && (p.ID == spec || strings.EqualFold(p.Name, spec)) {
			return p, nil
		}
	}
	return proto.TeamProject{}, fmt.Errorf("team: no project %q (\"enowx-app team projects\" lists them)", spec)
}

// quoteArg quotes an argument with spaces, for a command to copy.
func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t") {
		return `"` + s + `"`
	}
	return s
}
