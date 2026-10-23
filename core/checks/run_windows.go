//go:build windows

package checks

import (
	"os/exec"
	"strconv"
)

func shellCommand(command string) *exec.Cmd { return exec.Command("cmd", "/C", command) }

func startGroup(cmd *exec.Cmd) error { return cmd.Start() }

// killGroup ends the command and what it started.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
