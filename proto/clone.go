package proto

// Cloning a Git repository into a new project, on the host: started, then
// followed by its events (or asked for), cancelled while it runs.
const (
	MethodProjectClone       = "project.clone"        // ProjectCloneParams → CloneStatus
	MethodProjectCloneStatus = "project.clone.status" // CloneRef → CloneStatus
	MethodProjectCloneCancel = "project.clone.cancel" // CloneRef
	MethodProjectClones      = "project.clones"       // → []CloneStatus, the running and the last ones
)

// EventProjectClone carries a clone's CloneStatus as it goes.
const EventProjectClone = "project.clone"

// ProjectCloneParams clones URL into Dir/Name (Name: the repository's
// when ""), on Branch (a branch or tag; "" its default), Depth commits
// deep (0: all), with its submodules or not.
type ProjectCloneParams struct {
	URL        string `json:"url"`
	Dir        string `json:"dir"`
	Name       string `json:"name,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Depth      int    `json:"depth,omitempty"`
	Submodules bool   `json:"submodules,omitempty"`
}

// CloneRef names a clone.
type CloneRef struct {
	ID string `json:"id"`
}

// Clone states.
const (
	CloneRunning  = "running"
	CloneDone     = "done"
	CloneFailed   = "failed"
	CloneCanceled = "canceled"
)

// CloneStatus is how a clone goes: its address (a token in it hidden),
// the folder, git's phase and percent and last line, and once done the
// project it made, or why it failed.
type CloneStatus struct {
	ID      string       `json:"id"`
	URL     string       `json:"url"`
	Path    string       `json:"path"`
	State   string       `json:"state"`
	Phase   string       `json:"phase,omitempty"`
	Percent int          `json:"percent,omitempty"`
	Line    string       `json:"line,omitempty"`
	Error   string       `json:"error,omitempty"`
	Project *ProjectInfo `json:"project,omitempty"`
}

// MethodProjectClonePrefs is what clones start from: the folder this host
// cloned into last, and the repositories cloned lately on any host
// (kept in the synced settings): → ClonePrefs.
const MethodProjectClonePrefs = "project.clone.prefs"

// ClonePrefs are the folder to clone into here ("" none yet; Default the
// one cloned into last on any host, home-relative) and the repositories
// cloned lately, the latest first.
type ClonePrefs struct {
	Dir     string   `json:"dir,omitempty"`
	Default string   `json:"default,omitempty"`
	URLs    []string `json:"urls"`
}
