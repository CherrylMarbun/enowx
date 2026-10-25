package quietcmd

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// Hide keeps cmd from opening a console window, keeping the flags it has.
func Hide(cmd *exec.Cmd) *exec.Cmd {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
	cmd.SysProcAttr.HideWindow = true
	return cmd
}
