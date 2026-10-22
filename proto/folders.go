package proto

// Folder browsing, for picking a project folder on any host.
const (
	MethodFolderList = "folder.list" // FolderListParams → FolderListing
	MethodFolderMake = "folder.make" // FolderRef → FolderRef (the new folder)
)

// FolderListParams asks for the folders in Path: "" or "~" is the home
// folder, and "~/x" is under it.
type FolderListParams struct {
	Path   string `json:"path"`
	Hidden bool   `json:"hidden,omitempty"` // include names starting with a dot
}

// FolderListing is a folder's subfolders.
type FolderListing struct {
	Path    string        `json:"path"`             // absolute and clean
	Parent  string        `json:"parent,omitempty"` // "" at the root
	Home    string        `json:"home"`
	Git     bool          `json:"git,omitempty"` // Path is in a git repository
	Folders []FolderEntry `json:"folders"`
	Files   int           `json:"files"` // how many files it holds besides
}

// FolderEntry is a subfolder.
type FolderEntry struct {
	Name string `json:"name"`
	Git  bool   `json:"git,omitempty"` // it is the top of a git repository
}

// FolderRef names a folder.
type FolderRef struct {
	Path string `json:"path"`
}
