// Package agent knows the coding agents enowX can start and how to hand
// them a first prompt.
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

// Preset is a known agent CLI.
type Preset struct {
	ID      string
	Name    string
	Command string
	// PromptArg reports that the CLI takes a first prompt as its last
	// argument and stays interactive. Others get the prompt typed in.
	PromptArg bool
}

// Presets are the built-in agents, in the order the UI lists them.
var Presets = []Preset{
	{ID: "claude", Name: "Claude Code", Command: "claude", PromptArg: true},
	{ID: "codex", Name: "Codex", Command: "codex", PromptArg: true},
	{ID: "opencode", Name: "OpenCode", Command: "opencode"},
	{ID: "gemini", Name: "Gemini CLI", Command: "gemini"},
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

// List describes every preset, plus Shell and Custom, with whether its
// command is on this host's PATH.
func List() []proto.AgentInfo {
	out := make([]proto.AgentInfo, 0, len(Presets)+2)
	for _, p := range Presets {
		_, err := exec.LookPath(p.Command)
		out = append(out, proto.AgentInfo{ID: p.ID, Name: p.Name, Command: p.Command, Installed: err == nil})
	}
	out = append(out,
		proto.AgentInfo{ID: Shell, Name: "Shell", Installed: true},
		proto.AgentInfo{ID: Custom, Name: "Custom command", Installed: true},
	)
	return out
}
