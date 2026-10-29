package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// Notes attached to projects of any host, and those projects' context,
// read-only (core/notes, core/projectctx).

const attachUsage = `usage: enowx-app notes attach NOTE --project P [--project P2 …] [--host H]
       enowx-app notes detach NOTE (--project P | --all)
       enowx-app notes projects NOTE
       enowx-app notes --project P [--host H]
P is a project's name, folder or "." (this session's project, else this folder);
H a host's name, when names are the same on several`

// multi is a flag given any number of times.
type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// noteProjects attaches a note (attach), detaches it (detach), or lists
// its projects (projects).
func (r runner) noteProjects(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var projects multi
	fs.Var(&projects, "project", "")
	host := fs.String("host", "", "")
	all := fs.Bool("all", false, "")
	arg, err := parseWithArg(fs, args)
	if err != nil || arg == "" {
		return errors.New(attachUsage)
	}
	if os.Getenv(proto.EnvNotesPropose) != "" && cmd != "projects" {
		return errors.New("notes: here the note's projects are the user's to change: say which to attach")
	}
	n, err := r.findNote(arg)
	if err != nil {
		return err
	}
	switch cmd {
	case "attach":
		if len(projects) == 0 {
			return errors.New(attachUsage)
		}
		var add []proto.NoteProject
		for _, spec := range projects {
			found, err := r.resolveProjects(spec, *host)
			if err != nil {
				return err
			}
			add = append(add, found...)
		}
		info, err := r.c.AttachNote(n.ID, add)
		if err != nil {
			return err
		}
		return r.printProjects(info)
	case "detach":
		var refs []string
		for _, spec := range projects {
			i := slices.IndexFunc(n.Projects, func(p proto.NoteProject) bool {
				return p.Ref == spec || strings.EqualFold(p.Name, spec) || p.Path == spec
			})
			if i < 0 {
				return fmt.Errorf("notes: %q isn't attached to %s", n.Title, spec)
			}
			refs = append(refs, n.Projects[i].Ref)
		}
		if len(refs) == 0 && !*all {
			return errors.New(attachUsage)
		}
		info, err := r.c.DetachNote(n.ID, refs, *all)
		if err != nil {
			return err
		}
		return r.printProjects(info)
	}
	return r.printProjects(n)
}

// printProjects lists a note's projects, each with where it is now.
func (r runner) printProjects(n proto.NoteInfo) error {
	if len(n.Projects) == 0 {
		fmt.Fprintf(r.out, "%q is attached to no project\n", n.Title)
		return nil
	}
	cats, _ := r.c.Catalogs()
	for _, p := range n.Projects {
		var where []string
		hostKnown := false
		for _, cat := range cats {
			hostKnown = hostKnown || cat.Host == p.Host
			for _, cp := range cat.Projects {
				if notes.IsProject(p, cat.Host, cp) {
					where = append(where, cat.Name+":"+cp.Path)
				}
			}
		}
		state := strings.Join(where, ", ")
		if len(where) == 0 {
			state = "not on " + p.HostName + " now (moved or removed)"
			if !hostKnown {
				state = "on " + p.HostName + ", not synced here yet"
			}
		}
		fmt.Fprintf(r.out, "%s\t%s\t%s\n", p.Name, p.Ref, state)
	}
	return nil
}

// resolveProjects are the projects spec names: ".", a folder, a name or
// a ref, of host when given, the same repository once.
func (r runner) resolveProjects(spec, host string) ([]proto.NoteProject, error) {
	cats, err := r.c.Catalogs()
	if err != nil {
		return nil, err
	}
	info, err := r.c.Info()
	if err != nil {
		return nil, err
	}
	if spec == "." {
		spec, host = r.hereProject(), info.ID
	}
	var out []proto.NoteProject
	var names []string
	for _, cat := range cats {
		if host != "" && cat.Host != host && !strings.EqualFold(cat.Name, host) {
			continue
		}
		for _, cp := range cat.Projects {
			names = append(names, cp.Name+" ("+cat.Name+")")
			np := notes.CatalogProject(cat, cp)
			match := np.Ref == spec || strings.EqualFold(cp.Name, spec) ||
				filepath.IsAbs(spec) && cp.Path == spec && (host != "" || cat.Host == info.ID)
			if match && !slices.ContainsFunc(out, func(o proto.NoteProject) bool { return o.Ref == np.Ref }) {
				out = append(out, np)
			}
		}
	}
	switch {
	case len(out) == 0:
		return nil, fmt.Errorf("notes: no project %q: the projects of the hosts are %s", spec, strings.Join(names, ", "))
	case len(out) > 1:
		var which []string
		for _, p := range out {
			which = append(which, p.Name+" on "+p.HostName)
		}
		return nil, fmt.Errorf("notes: %q is %s: say which with --host", spec, strings.Join(which, " and "))
	}
	return out, nil
}

// hereProject is this session's project's folder, else the current one.
func (r runner) hereProject() string {
	if id := os.Getenv("ENOWX_SESSION"); id != "" {
		if list, err := r.c.Sessions(); err == nil {
			for _, s := range list {
				if s.ID == id && s.Project != "" {
					return s.Project
				}
			}
		}
	}
	dir, _ := os.Getwd()
	return dir
}

// notesOfProject lists the notes attached to the project spec names.
func (r runner) notesOfProject(spec, host string) error {
	found, err := r.resolveProjects(spec, host)
	if err != nil {
		return err
	}
	list, err := r.notesSeen()
	if err != nil {
		return err
	}
	p := found[0]
	count := 0
	for _, n := range list {
		if slices.ContainsFunc(n.Projects, func(np proto.NoteProject) bool {
			return np.Ref == p.Ref || notes.IsProject(np, p.Host, proto.CatalogProject{Path: p.Path, Remote: p.Remote})
		}) {
			count++
			var others []string
			for _, np := range n.Projects {
				if np.Ref != p.Ref {
					others = append(others, np.Name+" · "+np.HostName)
				}
			}
			line := fmt.Sprintf("%s  %s", n.ID, n.Title)
			if len(others) > 0 {
				line += "  (also " + strings.Join(others, ", ") + ")"
			}
			fmt.Fprintln(r.out, line)
		}
	}
	if count == 0 {
		fmt.Fprintf(r.out, "no note is attached to %s yet\n", p.Name)
	}
	return nil
}

// attachToSession attaches note id to this session's project, as a note
// made from a project's session belongs to it.
func (r runner) attachToSession(id string) {
	if os.Getenv("ENOWX_SESSION") == "" {
		return
	}
	if found, err := r.resolveProjects(".", ""); err == nil {
		_, _ = r.c.AttachNote(id, found)
	}
}
