package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
)

// projects lists the host's projects, or arranges them as the session
// list's drag and menus do.
func (r runner) projects(args []string) error {
	if len(args) == 0 {
		list, err := r.c.Projects()
		if err != nil {
			return err
		}
		for _, p := range list {
			if p.Group != "" {
				fmt.Fprintf(r.out, "%s\t%s\n", p.Path, p.Group)
			} else {
				fmt.Fprintln(r.out, p.Path)
			}
		}
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "group":
		if len(rest) != 2 {
			return errors.New("usage: enowx-app projects group PATH GROUP")
		}
		path, err := r.projectPath(rest[0])
		if err != nil {
			return err
		}
		// Joining a group puts it after the group's projects.
		before, err := r.afterGroup(rest[1])
		if err != nil {
			return err
		}
		return r.c.ArrangeProject(path, rest[1], before)
	case "move", "move-group":
		fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		before := fs.String("before", "", "")
		arg, err := parseWithArg(fs, rest)
		if err != nil || arg == "" {
			return fmt.Errorf("usage: enowx-app projects %s %s [--before PATH]", cmd, map[string]string{"move": "PATH", "move-group": "GROUP"}[cmd])
		}
		if *before != "" {
			if *before, err = r.projectPath(*before); err != nil {
				return err
			}
		}
		if cmd == "move-group" {
			return r.c.MoveProjectGroup(arg, *before)
		}
		path, err := r.projectPath(arg)
		if err != nil {
			return err
		}
		group, err := r.groupOf(path)
		if err != nil {
			return err
		}
		return r.c.ArrangeProject(path, group, *before)
	case "rename-group":
		if len(rest) != 2 {
			return errors.New("usage: enowx-app projects rename-group FROM TO")
		}
		return r.c.RenameProjectGroup(rest[0], rest[1])
	}
	return errors.New("usage: enowx-app projects [group|move|move-group|rename-group ARGS]")
}

// projectPath is the project at path, as the host has it: a path relative
// to here works.
func (r runner) projectPath(path string) (string, error) {
	list, err := r.c.Projects()
	if err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(path)
	for _, p := range list {
		if p.Path == path || p.Path == abs || p.Name == path {
			return p.Path, nil
		}
	}
	return "", fmt.Errorf("no project %s: \"enowx-app projects\" lists them", path)
}

func (r runner) groupOf(path string) (string, error) {
	list, err := r.c.Projects()
	if err != nil {
		return "", err
	}
	for _, p := range list {
		if p.Path == path {
			return p.Group, nil
		}
	}
	return "", nil
}

// afterGroup is the first project after group's last, "" for last.
func (r runner) afterGroup(group string) (string, error) {
	list, err := r.c.Projects()
	if err != nil {
		return "", err
	}
	last := -1
	for i, p := range list {
		if group != "" && p.Group == group {
			last = i
		}
	}
	if last >= 0 && last+1 < len(list) {
		return list[last+1].Path, nil
	}
	return "", nil
}
