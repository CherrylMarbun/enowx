//go:build !linux

package display

import "fmt"

// Available reports whether this host can run displays, and if not, what
// it needs: displays run on Linux servers.
func Available() (bool, string) {
	return false, "needs a Linux host: apps here show their windows on this host's own screen"
}

// InstallCommand is "": nothing installs displays here.
func InstallCommand() string { return "" }

// Start fails: displays run on Linux.
func Start() (*Display, error) {
	_, why := Available()
	return nil, fmt.Errorf("%w: %s", ErrUnsupported, why)
}
