// Package gitclone clones a Git repository into a new folder with the
// host's git and its credentials (SSH keys, gh, credential helpers),
// reporting git's progress and explaining its failures.
package gitclone

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/enowx/enowx/internal/quietcmd"
)

// Options are a clone's: the repository, where it goes, and how much.
type Options struct {
	URL        string
	Path       string // the folder made for it
	Branch     string // a branch or a tag; "" for the default branch
	Depth      int    // 0 for the whole history
	Submodules bool
}

// Progress is how far a clone is: git's phase ("Receiving objects"), its
// percent, and the line it said last.
type Progress struct {
	Phase   string
	Percent int
	Line    string
}

var (
	scpLike   = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[^/].*$`) // git@github.com:user/repo.git
	percentRe = regexp.MustCompile(`^([A-Za-z ]+):\s+(\d+)%`)
)

// Valid says why url isn't a repository address git clones, nil when it
// is: https, http, ssh, git, file, or user@host:path.
func Valid(raw string) error {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return errors.New("type the repository's address")
	case scpLike.MatchString(raw):
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return errors.New("that isn't a repository address: https://github.com/user/repo.git or git@github.com:user/repo.git")
	}
	switch u.Scheme {
	case "https", "http", "ssh", "git":
		if u.Host == "" || strings.Trim(u.Path, "/") == "" {
			return errors.New("the address names no repository")
		}
	case "file":
	default:
		return fmt.Errorf("git doesn't clone %s addresses", u.Scheme)
	}
	return nil
}

// Name is the folder name a repository gets: the last part of its path
// without ".git".
func Name(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if i := strings.LastIndexAny(raw, "/:"); i >= 0 {
		raw = raw[i+1:]
	}
	raw = strings.TrimSuffix(raw, ".git")
	if raw == "" || raw == "." || raw == ".." {
		return "repo"
	}
	return raw
}

// Redact hides a password or token in an address, as https://user:***@host.
func Redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "***")
	} else if u.Scheme == "https" || u.Scheme == "http" {
		u.User = url.User("***") // a token as the user name
	}
	return u.String()
}

// Clone clones o.URL into o.Path, which must not exist or be empty,
// telling progress what git reports. Cancelling ctx stops it; a clone that
// fails or stops leaves no folder behind.
func Clone(ctx context.Context, o Options, progress func(Progress)) error {
	if err := Valid(o.URL); err != nil {
		return err
	}
	if entries, err := os.ReadDir(o.Path); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s exists and isn't empty: pick another folder or name", o.Path)
	} else if err == nil {
		_ = os.Remove(o.Path) // git makes it
	}
	if err := os.MkdirAll(filepath.Dir(o.Path), 0o755); err != nil {
		return err
	}
	args := []string{"clone", "--progress"}
	if o.Branch != "" {
		args = append(args, "--branch", o.Branch)
	}
	if o.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(o.Depth))
	}
	if o.Submodules {
		args = append(args, "--recurse-submodules")
		if o.Depth > 0 {
			args = append(args, "--shallow-submodules")
		}
	}
	args = append(args, "--", strings.TrimSpace(o.URL), o.Path)
	cmd := quietcmd.CommandContext(ctx, "git", args...)
	// Never wait on a prompt nobody sees: a password, or an unknown host's
	// key (accepted the first time, as ssh does when asked).
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never",
		"GIT_SSH_COMMAND="+sshCommand())
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gitclone: git: %w", err)
	}
	var said bytes.Buffer
	follow(io.TeeReader(stderr, &said), o.URL, progress)
	err = cmd.Wait()
	if err == nil {
		return nil
	}
	os.RemoveAll(o.Path)
	if ctx.Err() != nil {
		return context.Canceled
	}
	return explain(said.String(), o.URL)
}

// sshCommand is ssh as git runs it here: no questions, a new host's key
// taken the first time.
func sshCommand() string {
	if own := os.Getenv("GIT_SSH_COMMAND"); own != "" {
		return own
	}
	return "ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new"
}

// follow reads git's progress, its lines ended by \r or \n.
func follow(r io.Reader, raw string, progress func(Progress)) {
	sc := bufio.NewScanner(r)
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	for sc.Scan() {
		line := strings.TrimSpace(strings.ReplaceAll(sc.Text(), raw, Redact(raw)))
		line = strings.TrimPrefix(line, "remote: ")
		if line == "" || progress == nil {
			continue
		}
		p := Progress{Line: line}
		if m := percentRe.FindStringSubmatch(line); m != nil {
			p.Phase = strings.TrimSpace(m[1])
			p.Percent, _ = strconv.Atoi(m[2])
		}
		progress(p)
	}
}

// explain turns what git said when it failed into what to do.
func explain(said, raw string) error {
	said = strings.ReplaceAll(said, raw, Redact(raw))
	low := strings.ToLower(said)
	last := lastLine(said)
	switch {
	case strings.Contains(low, "permission denied (publickey") || strings.Contains(low, "host key verification failed"):
		return fmt.Errorf("the server refused this host's SSH key: add its key (~/.ssh/id_*.pub) to your account at the Git host, or clone the https address. git said: %s", last)
	case strings.Contains(low, "could not read username") || strings.Contains(low, "authentication failed") || strings.Contains(low, "terminal prompts disabled"):
		return fmt.Errorf("the repository needs signing in, and git on this host has no credentials for it: run \"gh auth login\" (GitHub) or set a credential helper in a terminal here, or clone the SSH address. git said: %s", last)
	case strings.Contains(low, "remote branch") && strings.Contains(low, "not found"):
		return fmt.Errorf("the repository has no branch or tag by that name. git said: %s", last)
	case strings.Contains(low, "repository not found") || strings.Contains(low, "does not appear to be a git repository") || strings.Contains(low, "not found"):
		return fmt.Errorf("no such repository, or it is private and this host isn't signed in to it. git said: %s", last)
	case strings.Contains(low, "could not resolve host") || strings.Contains(low, "unable to access") || strings.Contains(low, "connection timed out") || strings.Contains(low, "network is unreachable"):
		return fmt.Errorf("couldn't reach the Git host from this host: check its network. git said: %s", last)
	}
	if last == "" {
		return errors.New("git couldn't clone it")
	}
	return fmt.Errorf("git couldn't clone it: %s", last)
}

func lastLine(s string) string {
	lines := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == '\r' })
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" && !percentRe.MatchString(l) {
			return l
		}
	}
	return ""
}
