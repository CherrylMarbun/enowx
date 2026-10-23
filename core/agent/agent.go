// Package agent knows the coding agents enowX can start: how to hand them a
// first prompt, how to skip their permission prompts, and how to continue
// their last conversation after a restart.
package agent

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"time"

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
	// Aliases are other names the same program installs as; the first on
	// the PATH of Command and Aliases runs.
	Aliases []string
	// Args always follow Command ("chat" for Kiro).
	Args []string
	// Env is added to the agent's environment.
	Env []string
	// PromptFlag precedes the first prompt; nil passes it as the last
	// argument.
	PromptFlag []string
	// TypePrompt types the first prompt into the agent once its screen
	// settles, for agents that take none on their command line.
	TypePrompt bool
	// SkipPermissions makes the agent run every tool without asking;
	// SkipEnv does it through the environment instead, or as well.
	SkipPermissions []string
	SkipEnv         []string
	// AllowSkip offers skipping permissions as a mode to switch to later,
	// without starting in it, so the chat can switch to it.
	AllowSkip []string
	// Continue starts the agent on its most recent conversation in the
	// working directory, which is how sessions survive a daemon restart.
	Continue []string
	// Resume starts the agent on the conversation whose id follows it:
	// a session copied from another host goes on with its own, even when
	// others ran in the same folder since. Empty: Continue serves.
	Resume []string
	// RefusesRootSkip: the agent won't skip permissions as root.
	RefusesRootSkip bool
	// SignIn is the full command line that signs the agent in without a
	// browser on the host (a link and a code to open elsewhere), which is
	// what works on a server; SignInEnv is added to its environment.
	SignIn    []string
	SignInEnv []string
	// Interrupt stops what the agent is doing and gives the prompt back,
	// as its user would press it; "" is Ctrl+C.
	Interrupt string
	// Install is the official way to install the agent on macOS and Linux,
	// as a POSIX shell command run without root; "" when enowX doesn't
	// know one it has checked.
	Install string
	// Homepage is where the agent's documentation is, to install it from
	// when Install is empty.
	Homepage string
	// Namesake tells another program that installs under the same name
	// from the agent: it prints Says when run with Args.
	Namesake *Namesake
}

// Namesake is another program with an agent's name.
type Namesake struct {
	Args []string
	Says string
}

// Base is the agent whose ways a preset shares (its hooks, commands,
// settings, conversations): Claude Code's for its teams' preset.
func Base(id string) string {
	if id == "claude-agent-teams" {
		return "claude"
	}
	return id
}

// InterruptKey is what stops the agent's work: Interrupt, else Ctrl+C.
func (p Preset) InterruptKey() string {
	if p.Interrupt != "" {
		return p.Interrupt
	}
	return "\x03"
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
	// Resume continues the conversation of this id, where the agent can
	// (Preset.Resume); else the last one, as Continue.
	Resume string
	// Hooks report the agent's state to enowX (HookArgs).
	Hooks []string
}

// Argv returns the command line that starts the agent as l says.
func (p Preset) Argv(l Launch) []string {
	argv := append([]string{p.Program()}, p.Args...)
	switch {
	case l.Resume != "" && len(p.Resume) > 0:
		argv = append(append(argv, p.Resume...), l.Resume)
	case l.Continue || l.Resume != "":
		argv = append(argv, p.Continue...)
	}
	if l.SkipPermissions {
		argv = append(argv, p.SkipPermissions...)
	} else if l.AllowSkip {
		argv = append(argv, p.AllowSkip...)
	}
	argv = append(argv, l.Hooks...)
	if l.Prompt != "" && !l.Continue && l.Resume == "" && !p.TypePrompt {
		argv = append(append(argv, p.PromptFlag...), l.Prompt)
	}
	return argv
}

// Program is the name the agent runs as on this host: Command, else the
// first of Aliases on the PATH.
func (p Preset) Program() string {
	if _, err := exec.LookPath(p.Command); err != nil {
		for _, a := range p.Aliases {
			if _, err := exec.LookPath(a); err == nil {
				return a
			}
		}
	}
	return p.Command
}

// Installed reports whether the agent's program is on this host's PATH,
// and is the agent rather than a namesake.
func (p Preset) Installed() bool {
	for _, name := range append([]string{p.Command}, p.Aliases...) {
		if _, err := exec.LookPath(name); err == nil {
			return p.Namesake == nil || !p.Namesake.is(name)
		}
	}
	return false
}

// is reports whether the program name is the namesake.
func (n Namesake) is(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, name, n.Args...).CombinedOutput()
	return strings.Contains(string(out), n.Says)
}

// Environ is what the agent's environment gets, skipping permissions or
// not.
func (p Preset) Environ(skip bool) []string {
	env := slices.Clone(p.Env)
	if skip {
		env = append(env, p.SkipEnv...)
	}
	return env
}

// List describes every preset, plus Shell and Custom, with whether its
// command is on this host's PATH.
func List() []proto.AgentInfo {
	out := make([]proto.AgentInfo, 0, len(Presets)+2)
	for _, p := range Presets {
		out = append(out, proto.AgentInfo{
			ID: p.ID, Name: p.Name, Command: p.Program(), Installed: p.Installed(),
			CanSkipPermissions: len(p.SkipPermissions)+len(p.SkipEnv) > 0, Install: p.Install, Homepage: p.Homepage,
			SignIn: strings.Join(append(slices.Clone(p.SignInEnv), p.SignIn...), " "),
		})
	}
	return append(out,
		proto.AgentInfo{ID: Shell, Name: "Shell", Installed: true},
		proto.AgentInfo{ID: Custom, Name: "Custom command", Installed: true},
	)
}
