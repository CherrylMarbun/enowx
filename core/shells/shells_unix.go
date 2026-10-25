//go:build !windows

package shells

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Login is the user's login shell.
func Login() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/sh"
}

// candidates are the shells /etc/shells lists, then those on PATH it
// doesn't (a fish from Homebrew, say).
func candidates() []proto.ShellInfo {
	var out []proto.ShellInfo
	if f, err := os.Open("/etc/shells"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "/") {
				out = append(out, proto.ShellInfo{Path: line})
			}
		}
	}
	for _, name := range []string{"zsh", "bash", "fish", "nu", "xonsh", "elvish", "pwsh"} {
		if path, err := exec.LookPath(name); err == nil {
			out = append(out, proto.ShellInfo{Path: path})
		}
	}
	return out
}

// Argv starts the shell at path as a terminal's: a login shell, so it has
// the user's PATH as a terminal app gives it.
func Argv(path string) []string {
	switch filepath.Base(path) {
	case "xonsh":
		return []string{path, "--login"}
	case "elvish", "pwsh":
		return []string{path}
	}
	return []string{path, "-l"}
}
