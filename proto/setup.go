package proto

// Setup: what a host has (toolchains, agents, git), to bring a server to
// what this device has. See docs/setup.md.
const (
	MethodSetupScan = "setup.scan" // → SetupScan
)

// SetupScan is what a host has.
type SetupScan struct {
	OS      string `json:"os"`      // "linux", "darwin", "windows"
	Arch    string `json:"arch"`    // "amd64", "arm64"
	Distro  string `json:"distro"`  // /etc/os-release's ID: "ubuntu", "debian"; "" elsewhere
	Release string `json:"release"` // its VERSION_ID
	// Packages is the system's package manager: "apt-get", "dnf",
	// "zypper", "pacman", "apk", "brew"; "" for none known.
	Packages string `json:"packages,omitempty"`
	Root     bool   `json:"root,omitempty"` // the daemon runs as root: no sudo
	// Tools are the programs found, by name ("node", "go", "git",
	// "claude", …), with their version ("" when it doesn't say).
	Tools map[string]string `json:"tools"`
	// GitName and GitEmail are git's global user.
	GitName  string `json:"gitName,omitempty"`
	GitEmail string `json:"gitEmail,omitempty"`
	// GitHub says gh is signed in.
	GitHub bool `json:"github,omitempty"`
}
