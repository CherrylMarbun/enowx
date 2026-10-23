// Package review tells what a session's branch changed against its base,
// and commits, pushes, opens pull requests for, merges and discards it.
// Everything goes through the git and gh CLIs, as the user would run them.
package review

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// remoteTimeout bounds what talks to a remote (push, gh).
const remoteTimeout = 2 * time.Minute

// Where is the folder a session runs in and the project it belongs to.
type Where struct {
	Dir      string
	Project  string // the project folder; "" when the session has none
	Worktree bool   // Dir is a worktree of Project, not Project itself
}

// DefaultBase is what a session's changes are compared with: the branch of
// the project folder for a worktree, nothing (the last commit) otherwise.
func DefaultBase(w Where) string {
	if w.Worktree && w.Project != "" {
		return git.CurrentBranch(w.Project)
	}
	return ""
}

// Status tells where w's branch stands against base.
func Status(w Where, base string) (proto.ReviewStatus, error) {
	if _, err := git.TopLevel(w.Dir); err != nil {
		return proto.ReviewStatus{}, fmt.Errorf("review: %s is not a git repository", w.Dir)
	}
	st := proto.ReviewStatus{Dir: w.Dir, Branch: git.CurrentBranch(w.Dir), Base: base, Worktree: w.Worktree}
	branches, _ := git.Branches(w.Dir)
	st.Bases = slices.DeleteFunc(branches, func(b string) bool { return b == st.Branch })
	if base != "" {
		if out, err := git.Run(w.Dir, "rev-list", "--left-right", "--count", base+"...HEAD"); err == nil {
			if f := strings.Fields(out); len(f) == 2 {
				st.Behind, _ = strconv.Atoi(f[0])
				st.Ahead, _ = strconv.Atoi(f[1])
			}
		}
	}
	st.Uncommitted = changedFiles(w.Dir)
	if up, err := git.Run(w.Dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		st.Upstream = up
		if out, err := git.Run(w.Dir, "rev-list", "--count", "@{upstream}..HEAD"); err == nil {
			st.Unpushed, _ = strconv.Atoi(out)
		}
	}
	st.Remote = pushRemote(w.Dir)
	_, err := exec.LookPath("gh")
	st.HasGH = err == nil
	if w.Worktree && w.Project != "" && base != "" && st.Branch != "" && git.CurrentBranch(w.Project) == base {
		st.ProjectDirty = changedFiles(w.Project) > 0
		st.CanMerge = !st.ProjectDirty
	}
	return st, nil
}

// changedFiles counts the files of dir with changes not committed.
func changedFiles(dir string) int {
	out, err := git.Run(dir, "status", "--porcelain")
	if err != nil || out == "" {
		return 0
	}
	return strings.Count(out, "\n") + 1
}

// pushRemote is the remote a push goes to: the branch's, else origin,
// else the only one.
func pushRemote(dir string) string {
	if r, err := git.Run(dir, "config", "branch."+git.CurrentBranch(dir)+".remote"); err == nil && r != "" {
		return r
	}
	out, err := git.Run(dir, "remote")
	if err != nil || out == "" {
		return ""
	}
	remotes := strings.Split(out, "\n")
	if slices.Contains(remotes, "origin") || len(remotes) > 1 {
		return "origin"
	}
	return remotes[0]
}

// Commit commits every change in dir, untracked files included.
func Commit(dir, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("review: the commit message is empty")
	}
	if changedFiles(dir) == 0 {
		return errors.New("review: nothing to commit")
	}
	if _, err := git.Run(dir, "add", "--all"); err != nil {
		return fmt.Errorf("review: stage changes: %w", err)
	}
	if _, err := git.Run(dir, "commit", "--quiet", "-m", message); err != nil {
		return fmt.Errorf("review: commit: %w", err)
	}
	return nil
}

// Push pushes dir's branch, setting its upstream the first time.
func Push(dir string) error {
	branch := git.CurrentBranch(dir)
	if branch == "" {
		return errors.New("review: no branch is checked out")
	}
	remote := pushRemote(dir)
	if remote == "" {
		return errors.New("review: the repository has no remote to push to")
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	if _, err := run(ctx, dir, "git", "push", "--set-upstream", remote, branch); err != nil {
		return fmt.Errorf("review: push %s: %w", branch, err)
	}
	return nil
}

// PullRequest pushes dir's branch and opens a pull request into base with
// gh, titled and described from its commits when title is empty. It returns
// the pull request's URL.
func PullRequest(dir, base, title, body string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errors.New("review: opening a pull request needs the GitHub CLI (gh) on this host")
	}
	if err := Push(dir); err != nil {
		return "", err
	}
	args := []string{"pr", "create", "--head", git.CurrentBranch(dir)}
	if base != "" {
		args = append(args, "--base", base)
	}
	if title = strings.TrimSpace(title); title == "" {
		args = append(args, "--fill")
	} else {
		args = append(args, "--title", title, "--body", body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	out, err := run(ctx, dir, "gh", args...)
	if err != nil {
		return "", fmt.Errorf("review: open a pull request: %w", err)
	}
	// gh prints the URL last.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1]), nil
}

// Merge merges w's branch into base in the project folder, which must have
// base checked out and nothing uncommitted. A merge that conflicts is
// undone, so the project folder stays as it was.
func Merge(w Where, base string) error {
	st, err := Status(w, base)
	if err != nil {
		return err
	}
	switch {
	case !st.CanMerge && st.ProjectDirty:
		return fmt.Errorf("review: %s has uncommitted changes; commit or stash them first", filepath.Base(w.Project))
	case !st.CanMerge:
		return fmt.Errorf("review: the project folder doesn't have %s checked out", base)
	case st.Uncommitted > 0:
		return errors.New("review: commit the session's changes before merging them")
	case st.Ahead == 0:
		return fmt.Errorf("review: %s has nothing %s doesn't", st.Branch, base)
	}
	msg := fmt.Sprintf("Merge branch '%s'", st.Branch)
	if _, err := git.Run(w.Project, "merge", "--no-ff", "-m", msg, st.Branch); err != nil {
		_, _ = git.Run(w.Project, "merge", "--abort")
		return fmt.Errorf("review: merge %s into %s: %w", st.Branch, base, err)
	}
	return nil
}

// Discard removes the worktree w.Dir, which must be under root (where
// enowX keeps worktrees), and its branch with deleteBranch.
func Discard(root string, w Where, deleteBranch bool) error {
	if !w.Worktree || w.Project == "" {
		return errors.New("review: only a session's own worktree can be discarded")
	}
	rel, err := filepath.Rel(root, w.Dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("review: %s isn't a worktree enowX made", w.Dir)
	}
	branch := git.CurrentBranch(w.Dir)
	if _, err := git.Run(w.Project, "worktree", "remove", "--force", w.Dir); err != nil {
		if _, statErr := os.Stat(w.Dir); !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("review: remove worktree: %w", err)
		}
		_, _ = git.Run(w.Project, "worktree", "prune")
	}
	if deleteBranch && branch != "" {
		if _, err := git.Run(w.Project, "branch", "-D", branch); err != nil {
			return fmt.Errorf("review: delete branch %s: %w", branch, err)
		}
	}
	return nil
}

// run runs a command that may talk to a remote: it fails instead of asking
// for credentials, which nobody would see, and stops at ctx's deadline.
func run(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return string(out), nil
}
