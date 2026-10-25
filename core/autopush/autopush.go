// Package autopush keeps a project's work safe on its Git remote without
// the user pushing: it commits a snapshot of the working tree onto a
// branch of its own and pushes that branch alone. The user's branch,
// index and working tree are never touched: the snapshot is made in an
// index of its own, with git's plumbing.
package autopush

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/core/git"
)

// DefaultBranch is the branch snapshots go to unless the user names one.
const DefaultBranch = "enowx/autosync"

// protected are branches an autopush never writes, whatever it is told.
var protected = map[string]bool{"main": true, "master": true, "develop": true, "trunk": true}

// CheckBranch says why branch can't take snapshots of the repository at
// dir, nil when it can: a valid name, not a usual main branch, not the
// one checked out.
func CheckBranch(dir, branch string) error {
	switch {
	case !git.ValidBranchName(branch):
		return fmt.Errorf("autopush: %q isn't a branch name", branch)
	case protected[branch]:
		return fmt.Errorf("autopush: never to %s: pick a branch of its own, such as %s", branch, DefaultBranch)
	case git.CurrentBranch(dir) == branch:
		return fmt.Errorf("autopush: %s is the branch checked out: pick a branch of its own, such as %s", branch, DefaultBranch)
	}
	return nil
}

// Snapshot is a commit made of the working tree.
type Snapshot struct {
	Commit  string
	Files   int      // changed since the branch's last snapshot (or HEAD)
	Skipped []string // files of secrets left out by their names
	Changed bool     // false: the tree is the branch's already, no commit made
}

// Options are how a snapshot is made.
type Options struct {
	Branch string
	Host   string    // named in the commit message
	Now    time.Time // the commit's time; zero for now
	// Env adds to git's environment (credentials from the vault, say).
	Env []string
}

// Take commits the working tree of the repository at dir, as git add -A
// would stage it (what .gitignore ignores left out, and files of secrets
// by their names too), onto o.Branch, when it differs from the branch's
// last snapshot. The snapshot's parents are the branch's last one and the
// commit checked out, when the branch doesn't hold it yet. Secrets found
// in the files changed refuse it with a *SecretsError.
func Take(dir string, o Options) (Snapshot, error) {
	top, err := git.TopLevel(dir)
	if err != nil {
		return Snapshot{}, errors.New("autopush: not a git repository")
	}
	if o.Branch == "" {
		o.Branch = DefaultBranch
	}
	if err := CheckBranch(top, o.Branch); err != nil {
		return Snapshot{}, err
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	idx, err := os.CreateTemp("", "enowx-autopush-*.index")
	if err != nil {
		return Snapshot{}, err
	}
	idx.Close()
	os.Remove(idx.Name()) // git makes it
	defer os.Remove(idx.Name())
	env := append([]string{"GIT_INDEX_FILE=" + idx.Name()}, o.Env...)
	run := func(args ...string) (string, error) { return runEnv(top, env, args...) }

	head, _ := run("rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	tip, _ := run("rev-parse", "--verify", "--quiet", "refs/heads/"+o.Branch+"^{commit}")
	if head != "" {
		if _, err := run("read-tree", head); err != nil {
			return Snapshot{}, fmt.Errorf("autopush: %w", err)
		}
	}
	if _, err := run("add", "-A", "--", "."); err != nil {
		return Snapshot{}, fmt.Errorf("autopush: %w", err)
	}
	staged, err := run("ls-files", "-z", "--cached")
	if err != nil {
		return Snapshot{}, fmt.Errorf("autopush: %w", err)
	}
	var snap Snapshot
	for _, f := range strings.Split(staged, "\x00") {
		if f != "" && SecretFile(f) {
			snap.Skipped = append(snap.Skipped, f)
		}
	}
	if len(snap.Skipped) > 0 {
		args := append([]string{"rm", "-q", "--cached", "--ignore-unmatch", "--"}, snap.Skipped...)
		if _, err := run(args...); err != nil {
			return Snapshot{}, fmt.Errorf("autopush: %w", err)
		}
	}
	tree, err := run("write-tree")
	if err != nil {
		return Snapshot{}, fmt.Errorf("autopush: %w", err)
	}
	from := tip
	if from == "" {
		from = head
	}
	if from != "" {
		if t, _ := run("rev-parse", from+"^{tree}"); t == tree {
			snap.Commit = from
			return snap, nil
		}
	}
	changed, err := changedFiles(run, from, tree)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Files = len(changed)
	var found []Finding
	for _, f := range changed {
		data, err := os.ReadFile(filepath.Join(top, filepath.FromSlash(f)))
		if err != nil {
			continue // deleted
		}
		found = append(found, ScanSecrets(f, data)...)
	}
	if len(found) > 0 {
		return Snapshot{}, &SecretsError{Findings: found}
	}
	args := []string{"commit-tree", tree, "-m", Message(snap.Files, o.Host, o.Now)}
	if tip != "" {
		args = append(args, "-p", tip)
	}
	if head != "" && head != tip {
		if tip == "" {
			args = append(args, "-p", head)
		} else if _, err := run("merge-base", "--is-ancestor", head, tip); err != nil {
			args = append(args, "-p", head)
		}
	}
	commitEnv := append(env, "GIT_AUTHOR_DATE="+o.Now.Format(time.RFC3339), "GIT_COMMITTER_DATE="+o.Now.Format(time.RFC3339))
	if _, err := runEnv(top, nil, "config", "user.email"); err != nil {
		commitEnv = append(commitEnv, "GIT_AUTHOR_NAME=enowX autosync", "GIT_AUTHOR_EMAIL=autosync@enowx.invalid",
			"GIT_COMMITTER_NAME=enowX autosync", "GIT_COMMITTER_EMAIL=autosync@enowx.invalid")
	}
	commit, err := runEnv(top, commitEnv, args...)
	if err != nil {
		return Snapshot{}, fmt.Errorf("autopush: %w", err)
	}
	if _, err := run("update-ref", "-m", "enowx autosync", "refs/heads/"+o.Branch, commit, tip); err != nil {
		return Snapshot{}, fmt.Errorf("autopush: %w", err)
	}
	snap.Commit, snap.Changed = commit, true
	return snap, nil
}

// Message is a snapshot's commit message.
func Message(files int, host string, now time.Time) string {
	n := fmt.Sprintf("%d files", files)
	if files == 1 {
		n = "1 file"
	}
	parts := []string{"autosync: " + n}
	if host != "" {
		parts = append(parts, host)
	}
	parts = append(parts, now.Format("2006-01-02 15:04"))
	return strings.Join(parts, " · ")
}

// changedFiles lists the files differing between commit from ("" for
// none) and tree.
func changedFiles(run func(...string) (string, error), from, tree string) ([]string, error) {
	var out string
	var err error
	if from == "" {
		out, err = run("ls-tree", "-r", "-z", "--name-only", tree)
	} else {
		out, err = run("diff-tree", "-r", "-z", "--name-only", "--no-renames", from, tree)
	}
	if err != nil {
		return nil, fmt.Errorf("autopush: %w", err)
	}
	var files []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// runEnv runs git in dir with env added, returning its trimmed output.
func runEnv(dir string, env []string, args ...string) (string, error) {
	cmd := git.Command(dir, args...)
	cmd.Env = append(append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if msg := strings.TrimSpace(stderr.String()); msg != "" && errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %s", args[0], msg)
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
