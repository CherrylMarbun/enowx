//go:build !windows

package quietcmd

import "os/exec"

// Hide does nothing where programs open no console window.
func Hide(cmd *exec.Cmd) *exec.Cmd { return cmd }
