package cli

import (
	"errors"
	"fmt"

	"github.com/enowx/enowx/proto"
)

// shells lists the host's shells, the one its terminals start with
// marked, or picks it.
func (r runner) shells(args []string) error {
	var list proto.ShellList
	var err error
	switch {
	case len(args) == 0:
		list, err = r.c.Shells()
	case len(args) == 2 && args[0] == "use":
		path := args[1]
		if path == "login" {
			path = ""
		}
		list, err = r.c.SetShell(path)
	default:
		return errors.New("usage: enowx-app shells [use PATH|login]")
	}
	if err != nil {
		return err
	}
	for _, s := range list.Shells {
		mark := " "
		if list.Default == s.Path || list.Default == "" && s.Login {
			mark = "*"
		}
		note := ""
		if s.Login {
			note = "\tlogin shell"
		}
		fmt.Fprintf(r.out, "%s %s\t%s%s\n", mark, s.Name, s.Path, note)
	}
	return nil
}
