// Package worktree creates git worktrees for agents.
package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// BranchPrefix prefixes branches enowX names itself.
const BranchPrefix = "enowx/"

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Create adds a worktree of p.Repo under root, in a folder named p.Name,
// on p.Branch ("enowx/<Name>" when empty). A missing branch is created from
// p.Base (HEAD when empty). If the worktree already exists it is returned
// as is, so Create is safe to repeat.
func Create(root string, p proto.WorktreeCreateParams) (proto.WorktreeInfo, error) {
	if !validName.MatchString(p.Name) {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: invalid name %q", p.Name)
	}
	if p.Repo == "" {
		return proto.WorktreeInfo{}, errors.New("worktree: repo path is empty")
	}
	repo, err := git.TopLevel(p.Repo)
	if err != nil {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: %s is not a git repository: %w", p.Repo, err)
	}
	branch := p.Branch
	if branch == "" {
		branch = BranchPrefix + p.Name
	}
	if !git.ValidBranchName(branch) {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: invalid branch name %q", branch)
	}
	path := filepath.Join(root, repoKey(repo), p.Name)
	info := proto.WorktreeInfo{Path: path, Branch: branch}

	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return info, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: %w", err)
	}

	args := []string{"worktree", "add"}
	if git.BranchExists(repo, branch) {
		args = append(args, path, branch)
	} else {
		args = append(args, "-b", branch, path)
		if p.Base != "" {
			args = append(args, p.Base)
		}
	}
	if _, err := git.Run(repo, args...); err != nil {
		return proto.WorktreeInfo{}, fmt.Errorf("worktree: add %s: %w", branch, err)
	}
	return info, nil
}

// FolderName turns a branch name into a worktree folder name:
// "feat/user-search" becomes "feat-user-search".
func FolderName(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-._")
	if len(name) > 64 {
		name = name[:64]
	}
	if name == "" {
		name = "worktree"
	}
	return name
}

// repoKey names a repo's worktree folder: readable and unique per path.
func repoKey(repo string) string {
	sum := sha256.Sum256([]byte(repo))
	return filepath.Base(repo) + "-" + hex.EncodeToString(sum[:4])
}
