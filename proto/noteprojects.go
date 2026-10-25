package proto

import "time"

// Notes attach to projects, any number each, on any host: a plan for a
// frontend here and a backend on a server names both. The link lives in
// the note, so it syncs with it; a project is named by its Git remote
// when it has one (the same on every host it's cloned on), else by its
// host and folder.

// NoteProject is a project a note is attached to.
type NoteProject struct {
	// Ref names the project for good: "git:host/owner/repo" for a
	// repository, "path:<daemon ID>:<folder>" otherwise.
	Ref  string `json:"ref"`
	Name string `json:"name"`
	// Host is the daemon (DaemonInfo.ID) it was attached on, HostName its
	// name then, and Path its folder there.
	Host     string `json:"host,omitempty"`
	HostName string `json:"hostName,omitempty"`
	Path     string `json:"path,omitempty"`
	Remote   string `json:"remote,omitempty"`
}

// Attaching and the projects' context.
const (
	MethodNoteAttach     = "note.attach"     // NoteAttachParams → NoteInfo
	MethodNoteDetach     = "note.detach"     // NoteDetachParams → NoteInfo
	MethodProjectContext = "project.context" // ProjectContextParams → ProjectContext
	MethodProjectRead    = "project.read"    // ProjectReadParams → ProjectFile
)

// NoteAttachParams attaches note ID to Projects, besides those it has.
type NoteAttachParams struct {
	ID       string        `json:"id"`
	Projects []NoteProject `json:"projects"`
}

// NoteDetachParams detaches note ID from projects Refs, or from all.
type NoteDetachParams struct {
	ID   string   `json:"id"`
	Refs []string `json:"refs,omitempty"`
	All  bool     `json:"all,omitempty"`
}

// ProjectContextParams asks for the context of project Path on daemon
// Host ("" for this one); Ref names it for the snapshot kept when its
// host is out of reach.
type ProjectContextParams struct {
	Path string `json:"path"`
	Host string `json:"host,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

// ProjectContext is what an agent planning for a project needs first, a
// summary to read files from: never the files themselves.
type ProjectContext struct {
	Ref        string    `json:"ref"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Host       string    `json:"host"`
	HostName   string    `json:"hostName,omitempty"`
	Remote     string    `json:"remote,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	LastCommit string    `json:"lastCommit,omitempty"`
	Stack      []string  `json:"stack,omitempty"`
	Readme     string    `json:"readme,omitempty"`
	Tree       []string  `json:"tree,omitempty"`
	Taken      time.Time `json:"taken"`
	// Cached says its host was out of reach: this is the snapshot kept
	// when it last answered.
	Cached bool `json:"cached,omitempty"`
}

// ProjectReadParams reads File, relative to project Path on daemon Host
// ("" for this one), read-only.
type ProjectReadParams struct {
	Path string `json:"path"`
	File string `json:"file"`
	Host string `json:"host,omitempty"`
}

// ProjectFile is a project's file as read: its text, cut at a size.
type ProjectFile struct {
	File      string `json:"file"`
	Text      string `json:"text"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}
