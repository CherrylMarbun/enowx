//go:build !windows

package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

// useLoginPath adopts the PATH of the user's login shell. A daemon started
// by a desktop app inherits the launcher's short PATH, which misses agents
// installed with npm, Homebrew, mise and the like.
func useLoginPath(log *slog.Logger) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	const marker = "__ENOWX_PATH__"
	// -i loads rc files (nvm and friends live there); output around the
	// marker is ignored since rc files may print.
	out, err := exec.CommandContext(ctx, shell, "-ilc", "printf '"+marker+"%s"+marker+"' \"$PATH\"").Output()
	if err != nil {
		log.Warn("could not read the login shell's PATH", "shell", shell, "err", err)
		return
	}
	parts := strings.Split(string(out), marker)
	if len(parts) < 3 || parts[1] == "" {
		return
	}
	os.Setenv("PATH", parts[1])
}
