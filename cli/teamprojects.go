package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/enowx/enowx/cloud/api"
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
       enowx-app team kit rm TEAM KIND NAME
       enowx-app team kit template TEAM NAME LINK   (a project template: a repository new projects start from)`

// teamKitCmd lists a team's kit, or adds to it or takes from it.
func (r runner) teamKitCmd(args []string) error {
	if len(args) >= 4 && args[0] == "template" {
		team, err := r.findSpace(args[1])
		if err != nil || team == "" {
			return errors.Join(err, errors.New(teamKitUsage))
		}
		data, _ := json.Marshal(proto.KitTemplateData{Repo: args[3]})
		_, err = r.c.SaveToTeamKit(proto.TeamKitSaveParams{Team: team, Kind: proto.KitTemplate, Name: args[2], Data: data})
		return err
	}
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

// teamHostsCmd lists the hosts the teams' members share.
func (r runner) teamHostsCmd() error {
	list, err := r.c.TeamHosts()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Fprintln(r.out, "No member shares a host: \"enowx-app team share TEAM PROJECT...\" shares this one.")
		return nil
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "HOST\tOWNER\tADDRESS\tPROJECTS")
	for _, h := range list {
		var names []string
		for _, p := range h.Projects {
			names = append(names, p.Name)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Name, h.Owner, h.Addr, strings.Join(names, ", "))
	}
	return w.Flush()
}

// teamShareCmd shares projects of this host with a team, or none.
func (r runner) teamShareCmd(args []string) error {
	if len(args) == 0 {
		sh, err := r.c.HostShare()
		if err != nil {
			return err
		}
		if len(sh.Teams) == 0 {
			fmt.Fprintln(r.out, "This host shares nothing.")
		}
		for _, t := range sh.Teams {
			fmt.Fprintf(r.out, "%s: %s\n", t.Team, strings.Join(t.Projects, ", "))
		}
		switch {
		case sh.Err != "":
			fmt.Fprintln(r.out, sh.Err)
		case sh.Addr != "":
			fmt.Fprintln(r.out, "Members reach it at "+sh.Addr)
		}
		return nil
	}
	team, err := r.findSpace(args[0])
	if err != nil || team == "" {
		return errors.Join(err, errors.New("usage: enowx-app team share [TEAM [PROJECT...]]"))
	}
	var paths []string
	for _, p := range args[1:] {
		abs, _ := filepath.Abs(p)
		paths = append(paths, abs)
	}
	sh, err := r.c.SetHostShare(team, paths)
	if err != nil {
		return err
	}
	if sh.Err != "" {
		fmt.Fprintln(r.out, sh.Err)
	} else if len(paths) > 0 {
		fmt.Fprintln(r.out, "Shared; members reach this host at "+sh.Addr+" (its Tailscale node shared with their accounts).")
	}
	return nil
}

// teamAuditCmd prints a team's log, the latest first (admins).
func (r runner) teamAuditCmd(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: enowx-app team audit TEAM")
	}
	t, err := r.findTeam(strings.Join(args, " "))
	if err != nil {
		return err
	}
	var log []api.AuditEntry
	if err := r.c.CloudCall("GET", strings.Replace(api.PathAudit, "{team}", t.ID, 1), nil, &log); err != nil {
		return err
	}
	w := tabwriter.NewWriter(r.out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tWHO\tACTION\tDETAIL")
	for _, e := range log {
		who := e.Account.Name
		if e.Account.GitHub != "" {
			who = "@" + e.Account.GitHub
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.At.Local().Format("2006-01-02 15:04"), who, e.Action, e.Detail)
	}
	return w.Flush()
}

// teamChatCmd prints a team's chat, or sends to it.
func (r runner) teamChatCmd(args []string) error {
	usage := errors.New("usage: enowx-app team chat TEAM [--last N] | team chat TEAM --send TEXT")
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	last := fs.Int("last", 0, "")
	send := fs.String("send", "", "")
	rest, err := parseInterspersed(fs, args)
	if err != nil || len(rest) == 0 {
		return usage
	}
	team, err := r.findSpace(strings.Join(rest, " "))
	if err != nil || team == "" {
		return errors.Join(err, usage)
	}
	if *send != "" {
		_, err := r.c.SendChat(proto.ChatSendParams{Team: team, Text: *send})
		return err
	}
	_ = r.c.RefreshChat(team) // what came since the last sync
	list, err := r.c.TeamChat(team, *last)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Fprintln(r.out, "No message yet.")
	}
	for _, m := range list {
		fmt.Fprintf(r.out, "[%s] %s: %s\n", m.At.Local().Format("2006-01-02 15:04"), m.Name, m.Text)
		for _, mn := range m.Mentions {
			fmt.Fprintf(r.out, "    (%s %s)\n", mn.Kind, mn.Title)
		}
	}
	return nil
}

// teamChatTools read and write a team's chat, as MCP tools.
var teamChatTools = []mcpTool{
	{name: "team_chat_read", description: "A team's chat, its last messages: only when the user asks to read it.",
		props:    map[string]mcpProp{"team": {"string", "The team: its name or ID"}, "last": {"string", "How many of its last messages (default 100)"}},
		required: []string{"team"},
		args:     func(a map[string]any) []string { return withFlag([]string{"chat", str(a, "team")}, a, "last", "last") }},
	{name: "team_chat_send", description: "Send a message to a team's chat, as the user: only when the user asks, with the words they want said. A message that looks like it holds a secret isn't sent.",
		props:    map[string]mcpProp{"team": {"string", "The team: its name or ID"}, "text": {"string", "The message"}},
		required: []string{"team", "text"},
		args:     func(a map[string]any) []string { return []string{"chat", str(a, "team"), "--send", str(a, "text")} }},
}
