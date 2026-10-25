package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

const projectLinksUsage = `  copy P --to HOST [--path D] [--env]
                                   copy project P (its folder, a project's name, or . ) to another
                                   host of the app's list, remembering it came from here
  links P                          P's copies on other hosts, how each syncs and how it went
  origin P                         where P came from, and where it was copied
  sync P [--to HOST] [--now] [--on|--off] [--interval 5m|--realtime] [--direction one-way|two-way|reverse]
       [--include PATTERNS] [--exclude PATTERNS] [--deletes mirror|never] [--bandwidth KBS]
                                   a copy's sync on its own (--to picks the copy when there are several);
                                   --now syncs it now (the app, which reaches both hosts, must be open)
  conflicts P                      files changed on both copies since their last sync, kept as each has them
  autopush P [--on|--off] [--interval 30m] [--branch B] [--remote R] [--pr|--no-pr] [--now]
                                   push P's work on its own to a branch of its own (enowx/autosync),
                                   never touching your branch; --now pushes now
  history P                        what P's syncs and pushes did, the latest first`

// projectLinkCmd runs the project commands about copies on other hosts
// and auto pushes.
func (r runner) projectLinkCmd(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	to := fs.String("to", "", "")
	path := fs.String("path", "", "")
	env := fs.Bool("env", false, "")
	now := fs.Bool("now", false, "")
	on := fs.Bool("on", false, "")
	off := fs.Bool("off", false, "")
	interval := fs.String("interval", "", "")
	realtime := fs.Bool("realtime", false, "")
	direction := fs.String("direction", "", "")
	include := fs.String("include", "", "")
	exclude := fs.String("exclude", "", "")
	deletes := fs.String("deletes", "", "")
	bandwidth := fs.Int("bandwidth", -1, "")
	branch := fs.String("branch", "", "")
	remote := fs.String("remote", "", "")
	pr := fs.Bool("pr", false, "")
	noPR := fs.Bool("no-pr", false, "")
	arg, err := parseWithArg(fs, args)
	if err != nil {
		return errors.New(projectUsage)
	}
	if *on && *off {
		return errors.New("project: --on or --off, not both")
	}
	dir, err := r.projectFolder(arg)
	if err != nil {
		return err
	}
	switch cmd {
	case "copy":
		if *to == "" {
			return errors.New("usage: enowx-app project copy P --to HOST [--path D]")
		}
		res, err := r.c.CopyProjectToHost(proto.ProjectCopyParams{Project: dir, To: *to, Path: *path, Env: *env})
		if err != nil {
			return err
		}
		fmt.Fprintf(r.out, "Copied to %s:%s, linked to this copy (link %s): \"enowx-app project sync %s --on\" syncs them on their own.\n",
			res.Link.Target.Host, res.Path, res.Link.ID, arg)
		return nil
	case "autopush":
		return r.autopush(dir, autopushFlags{on: *on, off: *off, now: *now, pr: *pr, noPR: *noPR, interval: *interval, branch: *branch, remote: *remote, set: setFlags(fs)})
	}
	id, err := r.c.ProjectID(dir)
	if err != nil {
		return err
	}
	links, err := r.c.ProjectLinks(id.ID)
	if err != nil {
		return err
	}
	switch cmd {
	case "links", "origin":
		info, err := r.c.Info()
		if err != nil {
			return err
		}
		return printLinks(r.out, links, info.ID, cmd == "origin")
	case "conflicts":
		n := 0
		for _, l := range links {
			for _, c := range l.Status.Conflicts {
				gone := ""
				if c.Gone[0] {
					gone = " (deleted on " + l.Source.Host + ")"
				} else if c.Gone[1] {
					gone = " (deleted on " + l.Target.Host + ")"
				}
				fmt.Fprintf(r.out, "%s\t%s ↔ %s%s\n", c.Path, l.Source.Host, l.Target.Host, gone)
				n++
			}
		}
		if n == 0 {
			fmt.Fprintln(r.out, "No conflicts.")
		} else {
			fmt.Fprintln(r.out, "Settle them in the app: the project's Sync & push on its own…, or by making both copies the same.")
		}
		return nil
	case "history":
		list, err := r.c.ProjectHistory(id.ID)
		if err != nil {
			return err
		}
		for _, e := range list {
			state := "ok"
			if !e.OK {
				state = "failed: " + e.Err
			}
			fmt.Fprintf(r.out, "%s\t%s\t%s\t%s\n", e.Time.Local().Format("2006-01-02 15:04"), e.Kind, e.Summary, state)
		}
		return nil
	case "sync":
		l, err := pickLink(links, *to)
		if err != nil {
			return err
		}
		changed := false
		set := setFlags(fs)
		if *on || *off {
			l.Auto, changed = *on, true
		}
		if set["interval"] {
			d, err := parseInterval(*interval)
			if err != nil {
				return err
			}
			l.Interval, l.Realtime, changed = int(d/time.Second), false, true
		}
		if *realtime {
			l.Realtime, changed = true, true
		}
		if set["direction"] {
			l.Direction, changed = *direction, true
		}
		if set["include"] {
			l.Include, changed = splitPatterns(*include), true
		}
		if set["exclude"] {
			l.Exclude, changed = splitPatterns(*exclude), true
		}
		if set["deletes"] {
			l.Deletes, changed = *deletes, true
		}
		if *bandwidth >= 0 {
			l.Bandwidth, changed = *bandwidth, true
		}
		if changed {
			if _, err := r.c.SaveProjectLink(l.ProjectLink); err != nil {
				return err
			}
		}
		if *now {
			st, err := r.c.SyncProjectLinkNow(l.ID)
			if err != nil {
				return err
			}
			l = st
		}
		return printLinks(r.out, []proto.ProjectLinkState{l}, "", false)
	}
	return errors.New(projectUsage)
}

func setFlags(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

// projectFolder is the folder of project arg on the host: a folder, the
// name of one of its projects, or "." (or nothing) for the one here.
func (r runner) projectFolder(arg string) (string, error) {
	if arg == "" || arg == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		if top, err := git.TopLevel(wd); err == nil {
			return top, nil
		}
		return wd, nil
	}
	if !strings.ContainsAny(arg, `/\~`) {
		if list, err := r.c.Projects(); err == nil {
			for _, p := range list {
				if strings.EqualFold(p.Name, arg) {
					return p.Path, nil
				}
			}
		}
	}
	if strings.HasPrefix(arg, "~") {
		return arg, nil // the daemon finds the home folder
	}
	return filepath.Abs(arg)
}

// pickLink is the link to the copy on host to, or the only one.
func pickLink(links []proto.ProjectLinkState, to string) (proto.ProjectLinkState, error) {
	var names []string
	for _, l := range links {
		if to != "" && (strings.EqualFold(l.Target.Host, to) || strings.EqualFold(l.Source.Host, to) || l.ID == to) {
			return l, nil
		}
		names = append(names, l.Source.Host+" ↔ "+l.Target.Host)
	}
	switch {
	case len(links) == 0:
		return proto.ProjectLinkState{}, errors.New("project: no copy on another host yet: \"enowx-app project copy P --to HOST\" makes one")
	case to == "" && len(links) == 1:
		return links[0], nil
	case to == "":
		return proto.ProjectLinkState{}, fmt.Errorf("project: several copies (%s): --to HOST picks one", strings.Join(names, ", "))
	}
	return proto.ProjectLinkState{}, fmt.Errorf("project: no copy on %s (%s)", to, strings.Join(names, ", "))
}

// printLinks shows links: with origin, where the project came from (here
// being daemon here) and where it went.
func printLinks(w io.Writer, links []proto.ProjectLinkState, here string, origin bool) error {
	if origin {
		var from, to []string
		for _, l := range links {
			switch here {
			case l.Target.Daemon:
				from = append(from, l.Source.Host+":"+l.Source.Path)
			case l.Source.Daemon:
				to = append(to, l.Target.Host+":"+l.Target.Path)
			}
		}
		if len(from) == 0 && len(to) == 0 {
			fmt.Fprintln(w, "Not copied from or to another host.")
		}
		if len(from) > 0 {
			fmt.Fprintln(w, "Came from: "+strings.Join(from, ", "))
		}
		if len(to) > 0 {
			fmt.Fprintln(w, "Copied to: "+strings.Join(to, ", "))
		}
		return nil
	}
	if len(links) == 0 {
		fmt.Fprintln(w, "No copy on another host yet.")
		return nil
	}
	for _, l := range links {
		dir := l.Direction
		if dir == "" {
			dir = proto.SyncTwoWay
		}
		when := "by hand"
		switch {
		case l.Auto && l.Realtime:
			when = "on its own, as files change"
		case l.Auto:
			every := 300
			if l.Interval > 0 {
				every = l.Interval
			}
			when = "on its own, every " + (time.Duration(every) * time.Second).String()
		}
		fmt.Fprintf(w, "%s\t%s:%s → %s:%s\t%s, %s\n", l.ID, l.Source.Host, l.Source.Path, l.Target.Host, l.Target.Path, dir, when)
		switch {
		case l.Status.Last.IsZero():
			fmt.Fprintln(w, "\tnever synced")
		case l.Status.Err != "":
			fmt.Fprintf(w, "\tlast sync %s failed: %s\n", l.Status.Last.Local().Format("2006-01-02 15:04"), l.Status.Err)
		default:
			fmt.Fprintf(w, "\tlast sync %s: %d files sent, %d received\n", l.Status.Last.Local().Format("2006-01-02 15:04"), l.Status.Sent, l.Status.Received)
		}
		if n := len(l.Status.Conflicts); n > 0 {
			fmt.Fprintf(w, "\t%d files in conflict: enowx-app project conflicts\n", n)
		}
	}
	return nil
}

type autopushFlags struct {
	on, off, now, pr, noPR   bool
	interval, branch, remote string
	set                      map[string]bool
}

// autopush shows or sets the auto pushes of the project at dir, and pushes
// it now with --now.
func (r runner) autopush(dir string, f autopushFlags) error {
	id, err := r.c.ProjectID(dir)
	if err != nil {
		return err
	}
	list, err := r.c.Autopushes()
	if err != nil {
		return err
	}
	st := proto.AutopushStatus{AutopushSettings: proto.AutopushSettings{Project: id.ID, Name: filepath.Base(id.Path)}}
	for _, s := range list {
		if s.Project == id.ID {
			st = s
		}
	}
	s := st.AutopushSettings
	changed := f.on || f.off || f.pr || f.noPR || f.set["interval"] || f.set["branch"] || f.set["remote"]
	if f.on || f.off {
		s.On = f.on
	}
	if f.set["interval"] {
		d, err := parseInterval(f.interval)
		if err != nil {
			return err
		}
		s.Interval = int(d / time.Second)
	}
	if f.set["branch"] {
		s.Branch = f.branch
	}
	if f.set["remote"] {
		s.Remote = f.remote
	}
	if f.pr || f.noPR {
		s.PR = f.pr
	}
	if changed {
		s.Path = id.Path
		if st, err = r.c.SetAutopush(s); err != nil {
			return err
		}
	}
	if f.now {
		if st, err = r.c.AutopushNow(id.Path); err != nil {
			return err
		}
	}
	printAutopush(r.out, st)
	return nil
}

func printAutopush(w io.Writer, st proto.AutopushStatus) {
	branch := st.Branch
	if branch == "" {
		branch = "enowx/autosync"
	}
	if !st.On {
		fmt.Fprintf(w, "Auto push: off (to %s when on)\n", branch)
	} else {
		every := time.Duration(st.Interval) * time.Second
		if every <= 0 {
			every = 30 * time.Minute
		}
		by := st.HostName
		if st.Here {
			by = "this host"
		}
		fmt.Fprintf(w, "Auto push: on, every %s, to %s, by %s\n", every, branch, by)
	}
	if st.Visibility == "public" {
		fmt.Fprintln(w, "Warning: the repository is public: anyone sees the work in progress.")
	}
	if !st.LastPush.IsZero() {
		fmt.Fprintf(w, "Last pushed %s: %s (%d files)\n", st.LastPush.Local().Format("2006-01-02 15:04"), short(st.Commit), st.Files)
	}
	if st.Err != "" {
		fmt.Fprintln(w, "Last try failed: "+st.Err)
	}
	if st.BranchURL != "" {
		fmt.Fprintln(w, st.BranchURL)
	}
	if st.PRURL != "" {
		fmt.Fprintln(w, "Pull request: "+st.PRURL)
	}
}

func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}

// parseInterval reads "90s", "5m", "1h" or seconds.
func parseInterval(s string) (time.Duration, error) {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("project: an interval such as 30s, 5m or 1h, not %q", s)
	}
	return d, nil
}

func splitPatterns(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// projectLinkTools are copies on other hosts, their syncs and auto
// pushes, as MCP tools.
var projectLinkTools = []mcpTool{
	{name: "project_copy", description: "Copy a project of this host to another host of the user's enowX app (by its name there), without its sessions; the copy remembers it came from here, for syncs. Needs the app open.",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}, "to": {"string", "The host to copy to, as named in the app"},
			"path": {"string", "Its folder there (default: the same place under the home folder)"}, "env": {"boolean", "Copy .env files too"}},
		required: []string{"project", "to"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag(withFlag([]string{"copy", str(a, "project")}, a, "to", "to"), a, "path", "path"), a, "env", "env")
		}},
	{name: "project_links", description: "A project's copies on other hosts: where it came from and went, which way and how often each syncs, how the last sync went, files in conflict.",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}}, required: []string{"project"},
		args: func(a map[string]any) []string { return []string{"links", str(a, "project")} }},
	{name: "project_origin", description: "Where a project came from (the host and folder it was copied from), and where it was copied.",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}}, required: []string{"project"},
		args: func(a map[string]any) []string { return []string{"origin", str(a, "project")} }},
	{name: "project_sync", description: "Set how a project's copy on another host syncs on its own (on/off, interval or as files change, direction, files), and/or sync it now (needs the app open, reaching both hosts).",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}, "to": {"string", "The copy's host, when there are several"},
			"now": {"boolean", "Sync now"}, "on": {"boolean", "Sync on its own"}, "off": {"boolean", "Stop syncing on its own"},
			"interval": {"string", "How often: 30s, 5m, 1h"}, "realtime": {"boolean", "Sync as files change"},
			"direction": {"string", "one-way (source → copy), two-way, or reverse"}, "include": {"string", "Only these files: patterns, comma-separated"},
			"exclude": {"string", "Never these files: patterns, comma-separated"}, "deletes": {"string", "mirror or never"}},
		required: []string{"project"},
		args: func(a map[string]any) []string {
			args := []string{"sync", str(a, "project")}
			for _, k := range []string{"to", "now", "on", "off", "interval", "realtime", "direction", "include", "exclude", "deletes"} {
				args = withFlag(args, a, k, k)
			}
			return args
		}},
	{name: "project_conflicts", description: "Files changed on both copies of a project since their last sync, kept as each has them until the user picks.",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}}, required: []string{"project"},
		args: func(a map[string]any) []string { return []string{"conflicts", str(a, "project")} }},
	{name: "project_autopush", description: "A project's auto pushes: its work committed to a branch of its own (enowx/autosync by default) and pushed at an interval, never touching the user's branch, staged files or working tree; secrets stop a push. Shows them, sets them, or with now pushes now (do it when your work is done, so it's safe on the remote).",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}, "on": {"boolean", "Turn auto pushes on"}, "off": {"boolean", "Turn them off"},
			"interval": {"string", "How often: 15m, 1h"}, "branch": {"string", "The branch pushed to"}, "now": {"boolean", "Push now"}, "pr": {"boolean", "Open a pull request of the branch"}},
		required: []string{"project"},
		args: func(a map[string]any) []string {
			args := []string{"autopush", str(a, "project")}
			for _, k := range []string{"on", "off", "interval", "branch", "now", "pr"} {
				args = withFlag(args, a, k, k)
			}
			return args
		}},
	{name: "project_history", description: "What a project's syncs and auto pushes did, done or failed with why, the latest first.",
		props: map[string]mcpProp{"project": {"string", "The project: its folder, or its name"}}, required: []string{"project"},
		args: func(a map[string]any) []string { return []string{"history", str(a, "project")} }},
}
