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
	// created, changes title or exits.
	EventSessionUpdated = "session.updated"
	// EventSessionRemoved carries a SessionRef after a session is killed.
	EventSessionRemoved = "session.removed"
)

// DaemonInfo describes the running daemon.
type DaemonInfo struct {
	Version  string    `json:"version"`
	PID      int       `json:"pid"`
	Started  time.Time `json:"started"`
	Sessions int       `json:"sessions"`
}

// SessionInfo describes a session.
type SessionInfo struct {
	ID       string    `json:"id"`
	Command  []string  `json:"command"`
	Dir      string    `json:"dir"`
	Title    string    `json:"title"`
	Created  time.Time `json:"created"`
	Exited   bool      `json:"exited"`
	ExitCode int       `json:"exitCode"`
}

// SessionCreateParams starts a session. An empty Command runs the login shell.
type SessionCreateParams struct {
	Command []string `json:"command,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Env     []string `json:"env,omitempty"`
	Cols    int      `json:"cols,omitempty"`
	Rows    int      `json:"rows,omitempty"`
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

// WorktreeCreateParams creates (or reuses) a git worktree of Repo named Name,
// on branch "enowx/<Name>" from Base (the repo's HEAD when empty).
type WorktreeCreateParams struct {
	Repo string `json:"repo"`
	Name string `json:"name"`
	Base string `json:"base,omitempty"`
}

// WorktreeInfo describes a worktree.
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}
