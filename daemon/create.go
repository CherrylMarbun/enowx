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
	// pasteSettle is how long a pasted prompt shows before Enter sends it.
	pasteSettle = 300 * time.Millisecond
)

// createSession resolves where and what to run for p, then starts it.
func (s *Server) createSession(p proto.SessionCreateParams) (*session.Session, error) {
	meta := session.Meta{Name: strings.TrimSpace(p.Name), Prompt: p.Prompt, Project: p.Project, Agent: p.Agent}
	setup := "" // the project's worktree setup, to run in a tab once started
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

	if p.Parent != "" {
		// A tab: the parent's folder, whatever it is.
		parent, err := s.sessions.Get(p.Parent)
		if err != nil {
			return nil, err
		}
		pm := parent.Meta()
		dir, meta.Parent = parent.Info().Dir, p.Parent
		meta.Project, meta.Branch, meta.Worktree = pm.Project, pm.Branch, pm.Worktree
	} else if p.Project != "" {
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
			if wt.Created {
				setup = s.readyWorktree(proj.Path, wt.Path, p.RunSetup)
			}
		default:
			return nil, fmt.Errorf("unknown branch mode %q", p.BranchMode)
		}
	}

	meta.Restart, meta.Relaunch, meta.SkipPermissions = launch.restart, launch.relaunch, launch.skip
	meta.Budget = time.Duration(p.Budget) * time.Second
	spec := session.Spec{
		ID:      session.NewID(),
		Command: launch.command, Dir: dir, Env: append(append(slices.Clone(p.Env), launch.env...), s.sessionEnv()...), Cols: p.Cols, Rows: p.Rows, Meta: meta,
	}
	if p.Display {
		disp, err := s.displays.start(spec.ID)
		if err != nil {
			return nil, err
		}
		spec.Env = append(spec.Env, disp.Env()...)
		spec.Meta.Display, spec.Meta.DisplayScale = disp.Port, disp.Scale
	}
	if _, isAgent := agent.Find(p.Agent); isAgent && !p.SignIn && s.mustQueue() {
		return s.enqueue(spec, launch.command, launch.typed)
	}
	sess, err := s.sessions.Create(spec)
	if err != nil {
		s.displays.stop(spec.ID)
		return nil, err
	}
	s.began(sess, launch.command, launch.typed)
	if setup != "" {
		s.runSetup(sess, setup)
	}
	return sess, nil
}

// readyWorktree gives a worktree just made from the project at repo what
// its .enowx.toml says: the ignored files to carry over, now; and its
// setup script, returned to run once the session started when runSetup.
func (s *Server) readyWorktree(repo, wt string, runSetup bool) string {
	cfg, err := project.LoadConfig(repo)
	if err != nil {
		s.logger().Warn("could not read the project's settings", "project", repo, "err", err)
		return ""
	}
	if copied, err := cfg.CopyInto(repo, wt); err != nil {
		s.logger().Warn("could not copy files into a new worktree", "worktree", wt, "err", err)
	} else if len(copied) > 0 {
		s.logger().Info("copied into a new worktree", "worktree", wt, "files", strings.Join(copied, ", "))
	}
	if !runSetup {
		return ""
	}
	return cfg.Worktree.Setup
}

// runSetup runs a worktree's setup script in a tab of sess, named Setup,
// which keeps a shell once it ends, so its output stays to read.
func (s *Server) runSetup(sess *session.Session, script string) {
	_, err := s.createSession(proto.SessionCreateParams{
		Parent: sess.ID(), Name: "Setup", Agent: agent.Custom, Command: thenShell(scriptArgv(script)),
	})
	if err != nil {
		s.logger().Warn("could not run the worktree's setup", "session", sess.ID(), "err", err)
	}
}

// sessionEnv is added to every session's environment: the variables the
// user set for this host, and the program the agents' hooks run to report
// their state (agent.EnvBin).
func (s *Server) sessionEnv() []string {
	env := s.vars.env() // the user's variables for this host
	if s.HookBin != "" {
		env = append(env, agent.EnvBin+"="+s.HookBin)
	}
	return env
}

// Bracketed paste marks typed text as pasted, for programs that read it.
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

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
	skip := p.SkipPermissions && len(preset.SkipPermissions)+len(preset.SkipEnv) > 0
	if skip && preset.RefusesRootSkip && os.Geteuid() == 0 {
		return launch{}, fmt.Errorf("%s refuses to skip permission prompts as root; reconnect and let enowX run as a regular user (Manage hosts → Ask user again), or turn off Skip permission prompts", preset.Name)
	}
	first := agentLaunch(preset, skip, prompt, false)
	if p.Resume != "" {
		first.Resume = p.Resume
	}
	l := launch{
		command:  thenShell(preset.Argv(first)),
		env:      preset.Environ(skip),
		restart:  thenShell(preset.Argv(agentLaunch(preset, skip, "", true))),
		relaunch: len(preset.Continue) > 0,
		skip:     skip,
	}
	if preset.TypePrompt && prompt != "" {
		// Pasted, so that its lines don't send it early.
		l.typed = pasteStart + prompt + pasteEnd
	}
	return l, nil
}

// agentLaunch is how this daemon starts preset: with its hooks and guide,
// so a session continued after an upgrade gets the current ones.
func agentLaunch(preset agent.Preset, skip bool, prompt string, cont bool) agent.Launch {
	// Skipping stays a mode to switch to from the chat, except as root,
	// where the agent would refuse to start with it.
	allow := !(preset.RefusesRootSkip && os.Geteuid() == 0)
	hooks := append(agent.HookArgs(preset.ID), agent.GuideArgs(preset.ID)...)
	return agent.Launch{Prompt: prompt, SkipPermissions: skip, AllowSkip: allow, Continue: cont, Hooks: hooks}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
