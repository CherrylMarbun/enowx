//go:build !windows

package daemon

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	os.Setenv("PATH", withUserBin(parts[1]))
}

// withUserBin adds ~/.local/bin, where agent installers put programs, when
// it exists but PATH lacks it: profiles add it only if it existed at login.
func withUserBin(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	bin := filepath.Join(home, ".local", "bin")
	if _, err := os.Stat(bin); err != nil || slices.Contains(filepath.SplitList(path), bin) {
		return path
	}
	return bin + string(filepath.ListSeparator) + path
}

var pathRefresh struct {
	sync.Mutex
	last time.Time
}

// refreshLoginPath reads the login shell's PATH again, at most every few
// seconds, so agents installed since the daemon started are found.
func refreshLoginPath(log *slog.Logger) {
	pathRefresh.Lock()
	defer pathRefresh.Unlock()
	if time.Since(pathRefresh.last) < 10*time.Second {
		return
	}
	pathRefresh.last = time.Now()
	useLoginPath(log)
}
