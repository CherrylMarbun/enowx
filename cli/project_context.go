package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/enowx/enowx/proto"
)

// projectContext prints a project's context (context P) or reads one of
// its files (read P FILE), read-only, on any host the app reaches.
func (r runner) projectContext(cmd string, args []string) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	host := fs.String("host", "", "")
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return errors.New("usage: enowx-app project context P [--host H] | project read P FILE [--host H]")
		}
		if fs.NArg() == 0 {
			break
		}
		pos, args = append(pos, fs.Arg(0)), fs.Args()[1:]
	}
	if len(pos) == 0 || cmd == "read" && len(pos) != 2 {
		return errors.New("usage: enowx-app project context P [--host H] | project read P FILE [--host H]")
	}
	found, err := r.resolveProjects(pos[0], *host)
	if err != nil {
		return err
	}
	p := found[0]
	if cmd == "read" {
		f, err := r.c.ReadProjectFile(proto.ProjectReadParams{Path: p.Path, File: pos[1], Host: p.Host})
		if err != nil {
			return err
		}
		fmt.Fprint(r.out, f.Text)
		if f.Truncated {
			fmt.Fprintf(r.out, "\n… (cut: %d bytes in all)\n", f.Size)
		}
		return nil
	}
	c, err := r.c.ProjectContext(proto.ProjectContextParams{Path: p.Path, Host: p.Host, Ref: p.Ref})
	if err != nil {
		return err
	}
	fmt.Fprintf(r.out, "# %s (%s on %s)\n", c.Name, c.Path, orName(c.HostName, p.HostName))
	if c.Cached {
		fmt.Fprintf(r.out, "(its host is out of reach: as it was on %s)\n", c.Taken.Local().Format("2006-01-02 15:04"))
	}
	for _, kv := range [][2]string{{"Remote", c.Remote}, {"Branch", c.Branch}, {"Last commit", c.LastCommit}, {"Stack", strings.Join(c.Stack, ", ")}} {
		if kv[1] != "" {
			fmt.Fprintf(r.out, "%s: %s\n", kv[0], kv[1])
		}
	}
	if len(c.Tree) > 0 {
		fmt.Fprintf(r.out, "\n## Files\n%s\n", strings.Join(c.Tree, "\n"))
	}
	if c.Readme != "" {
		fmt.Fprintf(r.out, "\n## README\n%s\n", c.Readme)
	}
	return nil
}

func orName(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// attachTools are notes' projects, and projects' context, as MCP tools.
var attachTools = []mcpTool{
	{name: "notes_attach", description: "Attach a note to one or more projects, of any host (a plan for a frontend here and a backend on a server names both).",
		props: map[string]mcpProp{"note": {"string", "The note's ID or title"}, "projects": {"array", "Projects: names, folders, or . for this session's"},
			"host": {"string", "A host's name, when a project's name is on several"}},
		required: []string{"note", "projects"},
		args: func(a map[string]any) []string {
			args := []string{"attach", str(a, "note")}
			list, _ := a["projects"].([]any)
			for _, p := range list {
				args = append(args, "--project", fmt.Sprint(p))
			}
			return withFlag(args, a, "host", "host")
		}},
	{name: "notes_detach", description: "Detach a note from a project, or from all (all=true); the note stays.",
		props:    map[string]mcpProp{"note": {"string", "The note's ID or title"}, "project": {"string", "The project's name"}, "all": {"boolean", "From every project"}},
		required: []string{"note"},
		args: func(a map[string]any) []string {
			return withFlag(withFlag([]string{"detach", str(a, "note")}, a, "project", "project"), a, "all", "all")
		}},
	{name: "notes_projects", description: "The projects a note is attached to, and where each is now (host and folder).",
		props: map[string]mcpProp{"note": {"string", "The note's ID or title"}}, required: []string{"note"},
		args: func(a map[string]any) []string { return []string{"projects", str(a, "note")} }},
	{name: "notes_for_project", description: "The notes attached to a project (its plans, TODOs): . for this session's project.",
		props: map[string]mcpProp{"project": {"string", "A project's name or folder, or ."}, "host": {"string", "A host's name"}}, required: []string{"project"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"--project", str(a, "project")}, a, "host", "host")
		}},
}

// contextTools read a project of any host, read-only.
var contextTools = []mcpTool{
	{name: "project_context", description: "A project's summary, of any host, read-only: stack, branch, last commit, its folders and README; read files after with project_read.",
		props: map[string]mcpProp{"project": {"string", "A project's name or folder"}, "host": {"string", "A host's name"}}, required: []string{"project"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"context", str(a, "project")}, a, "host", "host")
		}},
	{name: "project_read", description: "A file of a project, of any host, read-only (text, cut when long).",
		props:    map[string]mcpProp{"project": {"string", "A project's name or folder"}, "file": {"string", "The file, relative to the project"}, "host": {"string", "A host's name"}},
		required: []string{"project", "file"},
		args: func(a map[string]any) []string {
			return withFlag([]string{"read", str(a, "project"), str(a, "file")}, a, "host", "host")
		}},
}
