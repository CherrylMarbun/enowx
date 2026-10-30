//go:build windows

package hooks

import (
	"os/exec"
	"strconv"

	"github.com/enowx/enowx/internal/quietcmd"
)

func shellCommand(command string) *exec.Cmd { return quietcmd.Command("cmd", "/C", command) }

// killGroup ends the command and what it started.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = quietcmd.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
