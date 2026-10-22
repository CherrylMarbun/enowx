package daemon

import (
	"errors"
	"fmt"
	"path/filepath"
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
	meta := session.Meta{Name: p.Name, Project: p.Project, Agent: p.Agent}
	dir := p.Dir

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

	command, typed, err := commandFor(p)
	if err != nil {
		return nil, err
	}
	if meta.Name == "" {
		meta.Name = defaultName(p, meta, dir)
	}
	sess, err := s.sessions.Create(command, dir, p.Env, p.Cols, p.Rows, meta)
	if err != nil {
		return nil, err
	}
	if typed != "" {
		go func() {
			if sess.WaitQuiet(promptQuiet, promptTimeout) {
				sess.Terminal().Send([]byte(typed + "\r"))
			}
		}()
	}
	return sess, nil
}

// commandFor returns the command to run and, for agents that don't take a
// prompt argument, the prompt to type once they are ready.
func commandFor(p proto.SessionCreateParams) (command []string, typed string, err error) {
	prompt := strings.TrimSpace(p.Prompt)
	switch p.Agent {
	case "", agent.Custom:
		return p.Command, prompt, nil
	case agent.Shell:
		return nil, prompt, nil
	}
	preset, ok := agent.Find(p.Agent)
	if !ok {
		return nil, "", fmt.Errorf("unknown agent %q", p.Agent)
	}
	command = []string{preset.Command}
	if prompt != "" && preset.PromptArg {
		return append(command, prompt), "", nil
	}
	return command, prompt, nil
}

func defaultName(p proto.SessionCreateParams, meta session.Meta, dir string) string {
	switch {
	case meta.Worktree && meta.Branch != "":
		return meta.Branch
	case p.Agent == agent.Shell || (p.Agent == "" && len(p.Command) == 0):
		return "Shell"
	}
	if preset, ok := agent.Find(p.Agent); ok {
		return preset.Name
	}
	if len(p.Command) > 0 {
		return filepath.Base(p.Command[0])
	}
	return filepath.Base(dir)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
