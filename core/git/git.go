// Package git runs the git CLI, so behavior matches what users see in their
// own terminal.
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Command returns the git command for args in dir, to run with input or
// output of its own.
func Command(dir string, args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"-C", dir}, args...)...)
}

// Run runs git in dir and returns its trimmed stdout.
func Run(dir string, args ...string) (string, error) {
	cmd := Command(dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// TopLevel returns the root of the repository containing dir, or an error
// if dir is not in one. It is spelled as dir is: git resolves symlinks
// (macOS's /var is /private/var), which would make the root seem another
// folder than dir's ancestor.
func TopLevel(dir string) (string, error) {
	out, err := Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	top := filepath.Clean(out)
	abs, err := filepath.Abs(dir)
	if err != nil || abs == top || strings.HasPrefix(abs, top+string(filepath.Separator)) {
		return top, nil
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return top, nil
	}
	rel, err := filepath.Rel(top, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return top, nil
	}
	ancestor := abs
	if rel != "." {
		for range strings.Split(rel, string(filepath.Separator)) {
			ancestor = filepath.Dir(ancestor)
		}
	}
	if same, err := filepath.EvalSymlinks(ancestor); err == nil && same == top {
		return ancestor, nil
	}
	return top, nil
}

// CurrentBranch returns the branch checked out in dir ("" when detached).
func CurrentBranch(dir string) string {
	out, err := Run(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

// Branches lists the local branches, most recently committed first.
func Branches(dir string) ([]string, error) {
	out, err := Run(dir, "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// BranchExists reports whether the local branch exists.
func BranchExists(dir, branch string) bool {
	_, err := Run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// ValidBranchName reports whether git accepts name as a branch name.
func ValidBranchName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") {
		return false
	}
	return exec.Command("git", "check-ref-format", "--branch", name).Run() == nil
}
