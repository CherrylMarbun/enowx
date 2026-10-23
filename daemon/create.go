package daemon

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/core/worktree"
	"github.com/enowx/enowx/proto"
)

// How long a typed first prompt waits for the agent to show its input.
const (
	promptQuiet   = 1500 * time.Millisecond
	promptTimeout = 20 * time.Second
)

// createSession resolves where and what to run for p, then starts it.
func (s *Server) createSession(p proto.SessionCreateParams) (*session.Session, error) {
	meta := session.Meta{Name: strings.TrimSpace(p.Name), Prompt: p.Prompt, Project: p.Project, Agent: p.Agent}
	dir := p.Dir
	// Settle what to run before creating a worktree it would leave behind.
	launch, err := launchFor(p)
	if err != nil {
		return nil, err
	}
	if p.SignIn {
		// Signing in has nothing to do with branches: the project folder.
		p.BranchMode, meta.Prompt = proto.BranchNone, ""
	}
	if meta.Name == "" {
		meta.Name = launch.name
	}

	if p.Project != "" {
		proj := project.Inspect(p.Project)
		dir = proj.Path
		switch p.BranchMode {
		case proto.BranchNone:
			meta.Branch = proj.Branch
		case proto.BranchNew, proto.BranchExisting:
			if !proj.IsGit {
				return nil, fmt.Errorf("%s is not a git repository, so it has no branches", proj.Name)
			}
			branch := strings.TrimSpace(p.Branch)
			if branch == "" {
				return nil, errors.New("branch name is empty")
			}
			meta.Branch = branch
			if p.BranchMode == proto.BranchExisting && branch == proj.Branch {
				break // already checked out in the project folder
			}
			if p.BranchMode == proto.BranchNew && contains(proj.Branches, branch) {
				return nil, fmt.Errorf("branch %s already exists", branch)
			}
			if p.BranchMode == proto.BranchExisting && !contains(proj.Branches, branch) {
				return nil, fmt.Errorf("branch %s does not exist", branch)
			}
			wt, err := worktree.Create(s.WorktreeRoot, proto.WorktreeCreateParams{
				Repo: proj.Path, Name: worktree.FolderName(branch), Branch: branch, Base: p.Base,
			})
			if err != nil {
				return nil, err
			}
			dir, meta.Worktree = wt.Path, true
		default:
			return nil, fmt.Errorf("unknown branch mode %q", p.BranchMode)
		}
	}

	meta.Restart, meta.Relaunch, meta.SkipPermissions = launch.restart, launch.relaunch, launch.skip
	meta.Budget = time.Duration(p.Budget) * time.Second
	spec := session.Spec{
		Command: launch.command, Dir: dir, Env: append(append(slices.Clone(p.Env), launch.env...), s.sessionEnv()...), Cols: p.Cols, Rows: p.Rows, Meta: meta,
	}
	if _, isAgent := agent.Find(p.Agent); isAgent && !p.SignIn && s.mustQueue() {
		return s.enqueue(spec, launch.command, launch.typed)
	}
	sess, err := s.sessions.Create(spec)
	if err != nil {
		return nil, err
	}
	s.began(sess, launch.command, launch.typed)
	return sess, nil
}

// sessionEnv is added to every session's environment: the program the
// agents' hooks run to report their state (agent.EnvBin).
func (s *Server) sessionEnv() []string {
	if s.HookBin == "" {
		return nil
	}
	return []string{agent.EnvBin + "=" + s.HookBin}
}

// launch is how a session's program starts, now and after a restart.
type launch struct {
	command  []string // nil: the login shell
	env      []string // added to the session's environment
	name     string   // the session's name when it was given none
	typed    string   // prompt typed in once the program is ready
	restart  []string // command after a daemon restart
	relaunch bool     // restart the program at all after a daemon restart
	skip     bool     // the agent skips permission prompts
}

func launchFor(p proto.SessionCreateParams) (launch, error) {
	prompt := strings.TrimSpace(p.Prompt)
	if p.SignIn {
		preset, ok := agent.Find(p.Agent)
		if !ok || len(preset.SignIn) == 0 {
			return launch{}, fmt.Errorf("enowX doesn't know how to sign in to %q", p.Agent)
		}
		return launch{command: thenShell(preset.SignIn), env: preset.SignInEnv, name: "Sign in to " + preset.Name}, nil
	}
	switch p.Agent {
	case "", agent.Custom:
		// A custom command may not be safe to run twice, so it isn't.
		return launch{command: p.Command, typed: prompt}, nil
	case agent.Shell:
		return launch{typed: prompt, relaunch: true}, nil
	}
	preset, ok := agent.Find(p.Agent)
	if !ok {
		return launch{}, fmt.Errorf("unknown agent %q", p.Agent)
	}
	skip := p.SkipPermissions && len(preset.SkipPermissions) > 0
	if skip && preset.RefusesRootSkip && os.Geteuid() == 0 {
		return launch{}, fmt.Errorf("%s refuses to skip permission prompts as root; reconnect and let enowX run as a regular user (Manage hosts → Ask user again), or turn off Skip permission prompts", preset.Name)
	}
	// Skipping stays a mode to switch to from the chat, except as root,
	// where the agent would refuse to start with it.
	allow := !(preset.RefusesRootSkip && os.Geteuid() == 0)
	hooks := agent.HookArgs(preset.ID)
	return launch{
		command:  thenShell(preset.Argv(agent.Launch{Prompt: prompt, SkipPermissions: skip, AllowSkip: allow, Hooks: hooks})),
		restart:  thenShell(preset.Argv(agent.Launch{SkipPermissions: skip, AllowSkip: allow, Continue: true, Hooks: hooks})),
		relaunch: len(preset.Continue) > 0,
		skip:     skip,
	}, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
