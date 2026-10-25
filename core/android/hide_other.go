//go:build !windows

package android

import "os/exec"

// hideWindow does nothing where tools open no console.
func hideWindow(*exec.Cmd) {}
