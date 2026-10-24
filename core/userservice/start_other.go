//go:build !linux

package userservice

// Start does nothing: only Linux servers run the daemon as a service.
func Start(string) (bool, error) { return false, nil }
