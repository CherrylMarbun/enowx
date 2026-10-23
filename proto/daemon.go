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
	// Hash is the SHA-256 of the executable the daemon started from: the
	// code it runs, which a newer file on disk doesn't change. "" when
	// unknown (development daemons started without it, older ones).
	Hash string `json:"hash,omitempty"`
	// Display says sessions can run with a virtual display here;
	// DisplayNote, when they can't, what the host needs.
	Display     bool   `json:"display,omitempty"`
	DisplayNote string `json:"displayNote,omitempty"`
	// DisplayInstall is the command that installs what displays need,
	// when they can't run yet and the host's package manager is known.
	DisplayInstall string `json:"displayInstall,omitempty"`
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

// Settings of the daemon's host, kept by the daemon.
const (
	MethodDaemonSettings    = "daemon.settings"    // → DaemonSettings
	MethodDaemonSetSettings = "daemon.setSettings" // DaemonSettings → DaemonSettings
)

// DaemonSettings are the host's settings. MaxAgents is how many agents may
// work at once (0: no limit); sessions started past it wait, queued, for
// one to finish its turn.
type DaemonSettings struct {
	MaxAgents int `json:"maxAgents"`
}

// MethodSessionBudget sets how long a session's agent may work:
// SessionBudgetParams. Past it, the daemon interrupts the agent.
const MethodSessionBudget = "session.budget"

// SessionBudgetParams sets session ID's budget, in seconds; 0 lifts it.
type SessionBudgetParams struct {
	ID      string `json:"id"`
	Seconds int64  `json:"seconds"`
}
