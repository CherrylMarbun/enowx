package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/enowx/enowx/core/shells"
	"github.com/enowx/enowx/proto"
)

// The shell the host's terminals start with, which the user picks in
// Settings among the host's (core/shells), kept in <daemon dir>/shell.json.
// Terminals start it, and so do agents' sessions once the agent exits.

// chosenShell is the shell picked, "" for the login shell. One daemon runs
// per host, so it is the process's.
var chosenShell atomic.Pointer[string]

// defaultShell is the shell picked, or "" for the login shell.
func defaultShell() string {
	if p := chosenShell.Load(); p != nil {
		return *p
	}
	return ""
}

func (s *Server) shellFile() string { return filepath.Join(filepath.Dir(s.WorktreeRoot), "shell.json") }

func (s *Server) loadShell() {
	var st proto.ShellSetParams
	data, err := os.ReadFile(s.shellFile())
	if err == nil {
		_ = json.Unmarshal(data, &st)
	}
	chosenShell.Store(&st.Path)
}

func (s *Server) shellList() proto.ShellList {
	return proto.ShellList{Shells: shells.Available(), Default: defaultShell()}
}

func (s *Server) setShell(path string) error {
	if path != "" {
		if _, ok := shells.Find(path); !ok {
			return fmt.Errorf("%s is not a shell of this host", path)
		}
	}
	data, err := json.Marshal(proto.ShellSetParams{Path: path})
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.shellFile(), data, 0o600); err != nil {
		return fmt.Errorf("daemon: shell: %w", err)
	}
	chosenShell.Store(&path)
	return nil
}
