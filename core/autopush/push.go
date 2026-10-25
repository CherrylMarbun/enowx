package autopush

import (
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/internal/quietcmd"
)

// DefaultRemote is the remote pushed to unless the user names one.
const DefaultRemote = "origin"

// Push pushes branch, and nothing else, to remote; it may move the
// remote's branch anywhere (the branch is an autopush's own), never
// another.
func Push(dir, remote, branch string, env []string) error {
	if remote == "" {
		remote = DefaultRemote
	}
	if err := CheckBranch(dir, branch); err != nil {
		return err
	}
	ref := "refs/heads/" + branch
	if _, err := runEnv(dir, env, "push", "--quiet", "--no-verify", remote, "+"+ref+":"+ref); err != nil {
		return fmt.Errorf("autopush: push %s to %s: %w", branch, remote, err)
	}
	return nil
}

// RemoteURL is remote's URL in the repository at dir, its credentials
// cut, "" when it has none.
func RemoteURL(dir, remote string) string {
	if remote == "" {
		remote = DefaultRemote
	}
	out, err := git.Run(dir, "remote", "get-url", remote)
	if err != nil {
		return ""
	}
	if u, err := url.Parse(out); err == nil && u.User != nil {
		u.User = nil
		return u.String()
	}
	return out
}

var hostedRepo = regexp.MustCompile(`^(?:https?://|ssh://)?(?:[^@/]+@)?(github\.com|gitlab\.com|bitbucket\.org)[:/]([^/]+/[^/]+?)(?:\.git)?/?$`)

// BranchLink is the web page of branch on the hosting of remote URL u
// (GitHub, GitLab or Bitbucket), "" for others.
func BranchLink(u, branch string) string {
	m := hostedRepo.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	switch m[1] {
	case "gitlab.com":
		return "https://gitlab.com/" + m[2] + "/-/tree/" + branch
	case "bitbucket.org":
		return "https://bitbucket.org/" + m[2] + "/src/" + branch
	}
	return "https://github.com/" + m[2] + "/tree/" + branch
}

// Visibility is "public" or "private" for a GitHub repository at dir, as
// gh tells, "" when unknown (no gh, not signed in, not on GitHub).
func Visibility(dir string) string {
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}
	cmd := quietcmd.Command("gh", "repo", "view", "--json", "visibility", "-q", ".visibility")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(out)))
}

// OpenPullRequest opens a pull request of branch into base with gh,
// unless one is open; it returns its address.
func OpenPullRequest(dir, branch, base string) (string, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", errors.New("autopush: a pull request needs gh (GitHub's command line), signed in")
	}
	gh := func(args ...string) (string, error) {
		cmd := quietcmd.Command("gh", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	if out, err := gh("pr", "view", branch, "--json", "url,state", "-q", `select(.state == "OPEN") | .url`); err == nil && out != "" {
		return out, nil
	}
	args := []string{"pr", "create", "--head", branch, "--title", "Autosync from enowX", "--body",
		"Snapshots of the work in progress, pushed by enowX's autopush. Merge what you keep."}
	if base != "" {
		args = append(args, "--base", base)
	}
	out, err := gh(args...)
	if err != nil {
		return "", fmt.Errorf("autopush: pull request: %s", out)
	}
	lines := strings.Split(out, "\n")
	return lines[len(lines)-1], nil
}
