package proto

// Projects (folders the daemon knows) and their git worktrees.
const (
	MethodProjectList    = "project.list"    // → []ProjectInfo
	MethodProjectAdd     = "project.add"     // ProjectRef → ProjectInfo
	MethodProjectRemove  = "project.remove"  // ProjectRef
	MethodProjectInspect = "project.inspect" // ProjectRef → ProjectDetail
	MethodWorktreeCreate = "worktree.create" // WorktreeCreateParams → WorktreeInfo
)

// EventProjectsChanged carries the full []ProjectInfo after a change.
const EventProjectsChanged = "projects.changed"

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
	// Setup and Copy are what the project's .enowx.toml says new
	// worktrees get: a script to run, ignored files to carry over.
	Setup string   `json:"setup,omitempty"`
	Copy  []string `json:"copy,omitempty"`
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
	// Created says the worktree was made now, not there already.
	Created bool `json:"created,omitempty"`
}
