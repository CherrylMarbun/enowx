// Package worktree creates git worktrees for agents, using the git CLI so
// behavior matches what users see in their own terminal.
package worktree

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enowx/enowx/proto"
)

// BranchPrefix prefixes the branches enowX creates.
const BranchPrefix = "enowx/"

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Create adds a worktree of repo named name under root, on a new branch
// "enowx/<name>" from base (HEAD when empty). If the worktree already
// exists it is returned as is, so Create is safe to repeat.
func Create(root string, p proto.WorktreeCreateParams) (proto.WorktreeInfo, error) {
	if !validName.MatchString(p.Name) {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: invalid name %q", p.Name)
	}
	repo, err := topLevel(p.Repo)
	if err != nil {
		return proto.WorktreeInfo{}, err
	}
	branch := BranchPrefix + p.Name
	path := filepath.Join(root, repoKey(repo), p.Name)
	info := proto.WorktreeInfo{Path: path, Branch: branch}

	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return info, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: %w", err)
	}

	args := []string{"worktree", "add"}
	if branchExists(repo, branch) {
		args = append(args, path, branch)
	} else {
		args = append(args, "-b", branch, path)
		if p.Base != "" {
			args = append(args, p.Base)
		}
	}
	if _, err := git(repo, args...); err != nil {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: add %s: %w", p.Name, err)
	}
	return info, nil
}

// topLevel resolves dir to the root of its git repository.
func topLevel(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("worktree: repo path is empty")
	}
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("worktree: %s is not a git repository: %w", dir, err)
	}
	return filepath.Clean(out), nil
}

func branchExists(repo, branch string) bool {
	_, err := git(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// repoKey names a repo's worktree folder: readable and unique per path.
func repoKey(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return filepath.Base(repo) + "-" + hex.EncodeToString(sum[:4])
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
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
