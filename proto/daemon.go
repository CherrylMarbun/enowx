package proto

import "time"

// The daemon itself and the agents it can start.
const (
	MethodDaemonInfo = "daemon.info" // → DaemonInfo
	MethodAgentList  = "agent.list"  // → []AgentInfo
)

// DaemonInfo describes the running daemon.
type DaemonInfo struct {
	Version string `json:"version"`
	// Build is set by development builds only (see version.Build).
	Build    string    `json:"build,omitempty"`
	PID      int       `json:"pid"`
	Started  time.Time `json:"started"`
	Sessions int       `json:"sessions"`
	Hostname string    `json:"hostname"`
	Home     string    `json:"home"`
}

// AgentInfo describes an agent the daemon can start.
type AgentInfo struct {
	ID        string `json:"id"`      // "claude", "codex", "shell", "custom", …
	Name      string `json:"name"`    // "Claude Code"
	Command   string `json:"command"` // "claude"; empty for shell and custom
	Installed bool   `json:"installed"`
	// CanSkipPermissions: the agent has a mode that runs tools without asking.
	CanSkipPermissions bool `json:"canSkipPermissions,omitempty"`
	// Install is a shell command that installs the agent, "" if unknown.
	Install string `json:"install,omitempty"`
	// SignIn is the command that signs the agent in without a browser on
	// the host, as it reads; "" when there is none (or an older daemon).
	SignIn string `json:"signIn,omitempty"`
}
