package proto

// The shells a host has for its terminals, and the one they start with.
const (
	MethodShellList = "shell.list" // → ShellList
	MethodShellSet  = "shell.set"  // ShellSetParams → ShellList
)

// ShellInfo is a shell of the host.
type ShellInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Login says it is the user's login shell (on Windows, PowerShell),
	// what terminals start until the user picks another.
	Login bool `json:"login,omitempty"`
}

// ShellList is the host's shells and the one its terminals start with:
// Default, or the login shell when "".
type ShellList struct {
	Shells  []ShellInfo `json:"shells"`
	Default string      `json:"default,omitempty"`
}

// ShellSetParams picks the shell terminals start with; "" the login
// shell.
type ShellSetParams struct {
	Path string `json:"path,omitempty"`
}
