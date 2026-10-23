package proto

import "time"

// Files: the files of a session's folder, for the side panel's Files tool.
// Paths are relative to the session's folder ("" is the folder itself),
// and never reach out of it.
const (
	MethodFileList = "file.list" // FileRef → FileListing
	MethodFileRead = "file.read" // FileRef → FileContent
	// MethodFileUpload puts a file of the app's machine on the session's
	// host, in parts: FileUpload → FileUploaded. Files dropped on a
	// server's session go there, so its agent can read them.
	MethodFileUpload = "file.upload"
)

// UploadPart is the most a FileUpload carries at once.
const UploadPart = 4 << 20

// FileUpload is a part of a file sent to the session's host. The first
// part has no Path; the daemon answers with where the file goes, which
// the next parts give.
type FileUpload struct {
	Session string `json:"session"`
	Name    string `json:"name"` // the file's name, without folders
	Path    string `json:"path,omitempty"`
	Data    []byte `json:"data"`
}

// FileUploaded is where an uploaded file is, absolute on the host.
type FileUploaded struct {
	Path string `json:"path"`
}

// FileRef names a file or folder in a session's folder. Unless, when set,
// asks file.read for nothing when the file last changed then: Unchanged.
type FileRef struct {
	Session string    `json:"session"`
	Path    string    `json:"path"`
	Unless  time.Time `json:"unless,omitzero"`
	// Abs reads Path as an absolute path on the host, wherever it is: a
	// file a program printed (a screenshot in /tmp, say). file.read only.
	// Older daemons read it in the session's folder.
	Abs bool `json:"abs,omitempty"`
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

// FileContent is a file's text, an image's bytes, or why it shows none.
type FileContent struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified,omitzero"`
	Unchanged bool      `json:"unchanged,omitempty"` // since FileRef.Unless: nothing else is sent
	Text      string    `json:"text,omitempty"`
	Binary    bool      `json:"binary,omitempty"`
	Truncated bool      `json:"truncated,omitempty"` // longer than shown
	// Image is an image's format ("png", "svg", …) and Data its bytes,
	// for images up to MaxImage.
	Image string `json:"image,omitempty"`
	Data  []byte `json:"data,omitempty"`
}

// MaxImage is the largest image file.read sends.
const MaxImage = 8 << 20
