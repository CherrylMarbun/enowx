package proto

import (
	"encoding/json"
	"time"
)

// Hello is exchanged once in each direction when a connection opens.
type Hello struct {
	Protocol int    `json:"protocol"`
	Agent    string `json:"agent"` // "enowx/0.0.1" or "enowxd/0.0.1"
}

// Request is a client call. ID is unique per connection.
type Request struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Response answers the request with the same ID. Error is empty on success.
type Response struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Event is pushed by the daemon to every connection.
type Event struct {
	Name string          `json:"name"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Methods.
const (
	MethodDaemonInfo     = "daemon.info"
	MethodAgentList      = "agent.list"
	MethodProjectList    = "project.list"
	MethodProjectAdd     = "project.add"
	MethodProjectRemove  = "project.remove"
	MethodProjectInspect = "project.inspect"
	MethodSessionCreate  = "session.create"
	MethodSessionList    = "session.list"
	MethodSessionAttach  = "session.attach"
	MethodSessionDetach  = "session.detach"
	MethodSessionResize  = "session.resize"
	MethodSessionClear   = "session.clear"
	MethodSessionKill    = "session.kill"
	MethodWorktreeCreate = "worktree.create"
)

// Events.
const (
	// EventSessionUpdated carries a SessionInfo whenever a session is
	// created, changes title, starts or stops producing output, or exits.
	EventSessionUpdated = "session.updated"
	// EventSessionRemoved carries a SessionRef after a session is killed.
	EventSessionRemoved = "session.removed"
	// EventProjectsChanged carries the full []ProjectInfo after a change.
	EventProjectsChanged = "projects.changed"
)

// DaemonInfo describes the running daemon.
type DaemonInfo struct {
	Version  string    `json:"version"`
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
}

// ProjectInfo describes a folder registered as a project.
type ProjectInfo struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	IsGit  bool   `json:"isGit"`
	Branch string `json:"branch,omitempty"` // checked out in the project folder
}

// ProjectRef names a project by its path.
type ProjectRef struct {
	Path string `json:"path"`
}

// ProjectDetail is ProjectInfo plus what a new-session form needs.
type ProjectDetail struct {
	ProjectInfo
	Branches []string `json:"branches,omitempty"` // local branches
}

// Branch modes for SessionCreateParams.
const (
	BranchNone     = ""         // run in the project folder
	BranchNew      = "new"      // new branch in its own worktree
	BranchExisting = "existing" // existing branch, in a worktree unless checked out in the project folder
)

// SessionInfo describes a session.
type SessionInfo struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Project  string    `json:"project,omitempty"`
	Agent    string    `json:"agent,omitempty"`
	Branch   string    `json:"branch,omitempty"`
	Worktree bool      `json:"worktree,omitempty"` // Dir is a worktree, not the project folder
	Command  []string  `json:"command"`
	Dir      string    `json:"dir"`
	Title    string    `json:"title"`
	Created  time.Time `json:"created"`
	Active   bool      `json:"active"` // produced output in the last few seconds
	Exited   bool      `json:"exited"`
	ExitCode int       `json:"exitCode"`
}

// SessionCreateParams starts a session.
//
// With Project set, the daemon picks the directory from BranchMode: the
// project folder, a new worktree on a new branch, or an existing branch.
// Agent picks the command from the daemon's presets; "custom" (or an empty
// Agent) runs Command, and an empty Command runs the login shell. Prompt
// is passed to agents that take one, or typed once the agent is ready.
type SessionCreateParams struct {
	Project    string   `json:"project,omitempty"`
	Name       string   `json:"name,omitempty"`
	Agent      string   `json:"agent,omitempty"`
	BranchMode string   `json:"branchMode,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Base       string   `json:"base,omitempty"`
	Prompt     string   `json:"prompt,omitempty"`
	Command    []string `json:"command,omitempty"`
	Dir        string   `json:"dir,omitempty"`
	Env        []string `json:"env,omitempty"`
	Cols       int      `json:"cols,omitempty"`
	Rows       int      `json:"rows,omitempty"`
}

// SessionRef names a session.
type SessionRef struct {
	ID string `json:"id"`
}

// AttachResult names the stream that carries a session's frames.
type AttachResult struct {
	Stream uint32 `json:"stream"`
}

// StreamRef names a stream.
type StreamRef struct {
	Stream uint32 `json:"stream"`
}

// ResizeParams asks the daemon to resize the session behind a stream.
// Cell sizes are in pixels; zero keeps the previous ones.
type ResizeParams struct {
	Stream     uint32 `json:"stream"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
	CellWidth  int    `json:"cellWidth,omitempty"`
	CellHeight int    `json:"cellHeight,omitempty"`
}

// WorktreeCreateParams creates (or reuses) a git worktree of Repo in a
// folder named Name, on Branch (created from Base, or HEAD, if missing).
// An empty Branch means "enowx/<Name>".
type WorktreeCreateParams struct {
	Repo   string `json:"repo"`
	Name   string `json:"name"`
	Branch string `json:"branch,omitempty"`
	Base   string `json:"base,omitempty"`
}

// WorktreeInfo describes a worktree.
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}
