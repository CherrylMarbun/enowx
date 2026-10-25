package proto

import "time"

// Syncing a project brings two copies of it on two hosts (one made by a
// transfer, or cloned on each) to the same state, both ways
// (docs/transfer.md): each daemon says what its copy has (projectsync.scan),
// the app compares both with what they had at the last sync, and each side
// packs what the other lacks (projectsync.pack, carried with transfer.read
// and transfer.write) for the other to apply (projectsync.apply).
const (
	MethodProjectSyncScan  = "projectsync.scan"  // ProjectSyncScanParams → ProjectSyncState
	MethodProjectSyncPack  = "projectsync.pack"  // ProjectSyncPackParams → TransferInfo
	MethodProjectSyncApply = "projectsync.apply" // ProjectSyncApplyParams → ProjectSyncResult
)

// ProjectSyncScanParams asks for the state of the project at Project, its
// .env files counted only when Env says so.
type ProjectSyncScanParams struct {
	Project string `json:"project"`
	Env     bool   `json:"env,omitempty"`
	// A link's rules (ProjectLink): what .gitignore ignores too with All
	// (dependencies' folders still left out), only the files matching
	// Include when any, none matching Exclude; enowX's local folder never.
	All     bool     `json:"all,omitempty"`
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
	// Files alone: no branches or sessions.
	FilesOnly bool `json:"filesOnly,omitempty"`
}

// ProjectSyncState is a copy of a project as it is now: its files (what
// git doesn't ignore, without .git), its branches and its sessions.
type ProjectSyncState struct {
	Path     string            `json:"path"`
	Git      bool              `json:"git,omitempty"`
	Files    []SyncFile        `json:"files"`
	Branches map[string]string `json:"branches,omitempty"` // name → commit
	Sessions []SyncSession     `json:"sessions,omitempty"`
}

// SyncFile is a file of a copy: its path relative to its folder, with
// slashes, and a hash of its content (or of a symlink's target).
type SyncFile struct {
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

// SyncSession is a session of a copy. Key is the same for the session on
// both hosts: a transfer keeps when it was made and its agent.
type SyncSession struct {
	Key      string `json:"key"`
	ID       string `json:"id"`
	Parent   string `json:"parent,omitempty"` // its parent's key
	Name     string `json:"name,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Worktree bool   `json:"worktree,omitempty"`
	Running  bool   `json:"running,omitempty"`
	// Conversation is its agent's conversation (Claude Code's), "" for
	// none; Talk hashes it, Said is its size.
	Conversation string `json:"conversation,omitempty"`
	Talk         string `json:"talk,omitempty"`
	Said         int64  `json:"said,omitempty"`
	// Files are a worktree session's own folder's.
	Files []SyncFile `json:"files,omitempty"`
}

// ProjectSyncPackParams packs, from the project at Project, the files
// named, the branches named (only the commits the other side lacks: Have
// are the commits it has), and per session what the other side needs.
type ProjectSyncPackParams struct {
	Project  string            `json:"project"`
	Files    []string          `json:"files,omitempty"`
	Branches []string          `json:"branches,omitempty"`
	Have     []string          `json:"have,omitempty"`
	Sessions []SyncSessionPack `json:"sessions,omitempty"`
}

// SyncSessionPack is what goes of session ID: all of it for a session the
// other side doesn't have (New), else its conversation when Talk says so
// and its worktree's files named.
type SyncSessionPack struct {
	ID    string   `json:"id"`
	New   bool     `json:"new,omitempty"`
	Talk  bool     `json:"talk,omitempty"`
	Files []string `json:"files,omitempty"`
}

// ProjectSyncApplyParams applies archive ID, packed by projectsync.pack on
// the host named From, to the project at Project: its files written, the
// files in Delete deleted (and in DeleteIn, by session key, from the
// sessions' worktrees), its branches set to the commits in Branches when
// that only moves them forward (else kept beside them), its sessions
// updated or made.
type ProjectSyncApplyParams struct {
	ID       string              `json:"id"`
	From     string              `json:"from"`
	Project  string              `json:"project"`
	Delete   []string            `json:"delete,omitempty"`
	DeleteIn map[string][]string `json:"deleteIn,omitempty"`
	Branches map[string]string   `json:"branches,omitempty"`
}

// ProjectSyncResult is what applying did: sessions made or updated, and
// notes on what went otherwise (a branch kept beside one that differs).
type ProjectSyncResult struct {
	Sessions []SessionInfo `json:"sessions,omitempty"`
	Notes    []string      `json:"notes,omitempty"`
}
