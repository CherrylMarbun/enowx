//go:build !windows

package checks

import (
	"os/exec"
	"syscall"
)

func shellCommand(command string) *exec.Cmd { return exec.Command("/bin/sh", "-c", command) }

// startGroup starts cmd in a process group of its own, so killGroup takes
// what it starts (a test runner's workers) along.
func startGroup(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd.Start()
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
