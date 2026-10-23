package proto

// Files: the files of a session's folder, for the side panel's Files tool.
// Paths are relative to the session's folder ("" is the folder itself),
// and never reach out of it.
const (
	MethodFileList = "file.list" // FileRef → FileListing
	MethodFileRead = "file.read" // FileRef → FileContent
)

// FileRef names a file or folder in a session's folder.
type FileRef struct {
	Session string `json:"session"`
	Path    string `json:"path"`
}

// FileListing is a folder's entries: folders first, then files, by name.
type FileListing struct {
	Path      string      `json:"path"`
	Root      string      `json:"root"` // the session's folder, absolute
	Entries   []FileEntry `json:"entries"`
	Truncated bool        `json:"truncated,omitempty"` // more entries than shown
}

// FileEntry is a file or folder. Status is git's short status of a file
// ("M", "A", "D", "R", "?" for untracked), or "M" for a folder holding
// changes; Ignored marks what git ignores.
type FileEntry struct {
	Name    string `json:"name"`
	Dir     bool   `json:"dir,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Status  string `json:"status,omitempty"`
	Ignored bool   `json:"ignored,omitempty"`
}

// FileContent is a file's text, or why it shows none.
type FileContent struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Text      string `json:"text,omitempty"`
	Binary    bool   `json:"binary,omitempty"`
	Truncated bool   `json:"truncated,omitempty"` // longer than shown
}
