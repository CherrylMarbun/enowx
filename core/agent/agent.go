// Package agent knows the coding agents enowX can start: how to hand them a
// first prompt, how to skip their permission prompts, and how to continue
// their last conversation after a restart.
package agent

import (
	"os/exec"
	"slices"
	"strings"

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
	// AllowSkip offers skipping permissions as a mode to switch to later,
	// without starting in it, so the chat can switch to it.
	AllowSkip []string
	// Continue starts the agent on its most recent conversation in the
	// working directory, which is how sessions survive a daemon restart.
	Continue []string
	// RefusesRootSkip: the agent won't skip permissions as root.
	RefusesRootSkip bool
	// SignIn is the full command line that signs the agent in without a
	// browser on the host (a link and a code to open elsewhere), which is
	// what works on a server; SignInEnv is added to its environment.
	SignIn    []string
	SignInEnv []string
	// Install is the official way to install the agent on macOS and Linux,
	// as a POSIX shell command run without root; "" when enowX doesn't
	// know one it has checked.
	Install string
}

// Presets are the built-in agents, in the order the UI lists them.
var Presets = []Preset{
	{
		ID: "claude", Name: "Claude Code", Command: "claude",
		SkipPermissions: []string{"--dangerously-skip-permissions"},
		AllowSkip:       []string{"--allow-dangerously-skip-permissions"},
		Continue:        []string{"--continue"},
		RefusesRootSkip: true,
		SignIn:          []string{"claude", "auth", "login"},
		Install:         "curl -fsSL https://claude.ai/install.sh | bash",
	},
	{
		ID: "codex", Name: "Codex", Command: "codex",
		SkipPermissions: []string{"--dangerously-bypass-approvals-and-sandbox"},
		Continue:        []string{"resume", "--last"},
		SignIn:          []string{"codex", "login", "--device-auth"},
		Install:         "curl -fsSL https://chatgpt.com/codex/install.sh | sh",
	},
	{
		ID: "opencode", Name: "OpenCode", Command: "opencode",
		PromptFlag:      []string{"--prompt"},
		SkipPermissions: []string{"--auto"},
		Continue:        []string{"--continue"},
		SignIn:          []string{"opencode", "auth", "login"},
		Install:         "curl -fsSL https://opencode.ai/install | bash",
	},
	{
		ID: "gemini", Name: "Gemini CLI", Command: "gemini",
		PromptFlag:      []string{"--prompt-interactive"},
		SkipPermissions: []string{"--yolo"},
		Continue:        []string{"--resume", "latest"},
		// No login command: its first screen signs in, with a link and a
		// code instead of a browser when NO_BROWSER is set.
		SignIn:    []string{"gemini"},
		SignInEnv: []string{"NO_BROWSER=true"},
		// npm's global folder often needs root; ~/.local/bin doesn't.
		Install: `command -v npm >/dev/null 2>&1 || { echo "Gemini CLI installs with npm: install Node.js first (https://nodejs.org)." >&2; exit 1; }; ` +
			`npm install -g --prefix "$HOME/.local" @google/gemini-cli`,
	},
	{
		ID: "kiro", Name: "Kiro CLI", Command: "kiro-cli",
		Args:            []string{"chat"},
		SkipPermissions: []string{"--trust-all-tools"},
		Continue:        []string{"--resume"},
		SignIn:          []string{"kiro-cli", "login", "--use-device-flow"},
		Install:         "curl -fsSL https://cli.kiro.dev/install | bash",
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
	// AllowSkip offers skipping permissions as a mode for later (see
	// Preset.AllowSkip); it does nothing when SkipPermissions is set.
	AllowSkip bool
	Continue  bool // continue the last conversation instead of starting one
	// Hooks report the agent's state to enowX (HookArgs).
	Hooks []string
}

// Argv returns the command line that starts the agent as l says.
func (p Preset) Argv(l Launch) []string {
	argv := append([]string{p.Command}, p.Args...)
	if l.Continue {
		argv = append(argv, p.Continue...)
	}
	if l.SkipPermissions {
		argv = append(argv, p.SkipPermissions...)
	} else if l.AllowSkip {
		argv = append(argv, p.AllowSkip...)
	}
	argv = append(argv, l.Hooks...)
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
			CanSkipPermissions: len(p.SkipPermissions) > 0, Install: p.Install,
			SignIn: strings.Join(append(slices.Clone(p.SignInEnv), p.SignIn...), " "),
		})
	}
	return append(out,
		proto.AgentInfo{ID: Shell, Name: "Shell", Installed: true},
		proto.AgentInfo{ID: Custom, Name: "Custom command", Installed: true},
	)
}
