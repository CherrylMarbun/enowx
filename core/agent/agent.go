// Package agent knows the coding agents enowX can start: how to hand them a
// first prompt, how to skip their permission prompts, and how to continue
// their last conversation after a restart.
package agent

import (
	"os/exec"

	"github.com/enowx/enowx/proto"
)

// IDs of the built-in presets. Shell runs the login shell; Custom runs a
// command the user types.
const (
	Shell  = "shell"
	Custom = "custom"
)

// Preset is a known agent CLI. Where each flag comes from is in
// docs/agents.md.
type Preset struct {
	ID      string
	Name    string
	Command string
	// Args always follow Command ("chat" for Kiro).
	Args []string
	// PromptFlag precedes the first prompt; nil passes it as the last
	// argument.
	PromptFlag []string
	// SkipPermissions makes the agent run every tool without asking.
	SkipPermissions []string
	// Continue starts the agent on its most recent conversation in the
	// working directory, which is how sessions survive a daemon restart.
	Continue []string
	// RefusesRootSkip: the agent won't skip permissions as root.
	RefusesRootSkip bool
}

// Presets are the built-in agents, in the order the UI lists them.
var Presets = []Preset{
	{
		ID: "claude", Name: "Claude Code", Command: "claude",
		SkipPermissions: []string{"--dangerously-skip-permissions"},
		Continue:        []string{"--continue"},
		RefusesRootSkip: true,
	},
	{
		ID: "codex", Name: "Codex", Command: "codex",
		SkipPermissions: []string{"--dangerously-bypass-approvals-and-sandbox"},
		Continue:        []string{"resume", "--last"},
	},
	{
		ID: "opencode", Name: "OpenCode", Command: "opencode",
		PromptFlag:      []string{"--prompt"},
		SkipPermissions: []string{"--auto"},
		Continue:        []string{"--continue"},
	},
	{
		ID: "gemini", Name: "Gemini CLI", Command: "gemini",
		PromptFlag:      []string{"--prompt-interactive"},
		SkipPermissions: []string{"--yolo"},
		Continue:        []string{"--resume", "latest"},
	},
	{
		ID: "kiro", Name: "Kiro CLI", Command: "kiro-cli",
		Args:            []string{"chat"},
		SkipPermissions: []string{"--trust-all-tools"},
		Continue:        []string{"--resume"},
	},
}

// Find returns the preset with the given ID.
func Find(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// Launch is how to start an agent.
type Launch struct {
	Prompt          string // first prompt
	SkipPermissions bool
	Continue        bool // continue the last conversation instead of starting one
}

// Argv returns the command line that starts the agent as l says.
func (p Preset) Argv(l Launch) []string {
	argv := append([]string{p.Command}, p.Args...)
	if l.Continue {
		argv = append(argv, p.Continue...)
	}
	if l.SkipPermissions {
		argv = append(argv, p.SkipPermissions...)
	}
	if l.Prompt != "" && !l.Continue {
		argv = append(append(argv, p.PromptFlag...), l.Prompt)
	}
	return argv
}

// List describes every preset, plus Shell and Custom, with whether its
// command is on this host's PATH.
func List() []proto.AgentInfo {
	out := make([]proto.AgentInfo, 0, len(Presets)+2)
	for _, p := range Presets {
		_, err := exec.LookPath(p.Command)
		out = append(out, proto.AgentInfo{
			ID: p.ID, Name: p.Name, Command: p.Command, Installed: err == nil,
			CanSkipPermissions: len(p.SkipPermissions) > 0,
		})
	}
	return append(out,
		proto.AgentInfo{ID: Shell, Name: "Shell", Installed: true},
		proto.AgentInfo{ID: Custom, Name: "Custom command", Installed: true},
	)
}
