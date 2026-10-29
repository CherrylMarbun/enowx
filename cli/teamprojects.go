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

const teamKitUsage = `usage: enowx-app team kit [TEAM]
       enowx-app team kit add TEAM ITEM       (ITEM: skill NAME, command NAME, subagent NAME, or an ID as claude/skill/NAME)
       enowx-app team kit rm TEAM KIND NAME`

// teamKitCmd lists a team's kit, or adds to it or takes from it.
func (r runner) teamKitCmd(args []string) error {
	if len(args) == 0 || args[0] != "add" && args[0] != "rm" {
		kit, err := r.c.TeamKit()
		if err != nil {
			return err
		}
		team := ""
		if len(args) > 0 {
			if team, err = r.findSpace(strings.Join(args, " ")); err != nil {
				return err
			}
		}
		w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "KIND\tNAME\tABOUT\tPATH")
		n := 0
		for _, it := range kit {
			if team == "" || it.Team == team {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", it.Kind, it.Name, it.About, it.Path)
				n++
			}
		}
		if n == 0 {
			fmt.Fprintln(r.out, "No kit yet: \"enowx-app team kit add TEAM skill NAME\" shares one of your agents' skills.")
			return nil
		}
		return w.Flush()
	}
	if len(args) < 3 {
		return errors.New(teamKitUsage)
	}
	team, err := r.findSpace(args[1])
	if err != nil || team == "" {
		return errors.Join(err, errors.New(teamKitUsage))
	}
	if args[0] == "rm" {
		if len(args) < 4 {
			return errors.New(teamKitUsage)
		}
		return r.c.RemoveFromTeamKit(team, args[2], strings.Join(args[3:], " "))
	}
	id := args[2]
	if len(args) >= 4 {
		scan, err := r.c.EnvScan()
		if err != nil {
			return err
		}
		id = ""
		for _, it := range scan.Items {
			if it.Kind == args[2] && it.Name == args[3] && (id == "" || it.Agent == "claude") {
				id = it.ID
			}
		}
		if id == "" {
			return fmt.Errorf("team: no %s %q among this host's agents", args[2], args[3])
		}
	}
	it, err := r.c.AddToTeamKit(team, id)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "Added %s %s to the team's kit: sessions in the team's projects are told of it.\n", it.Kind, it.Name)
	return nil
}
