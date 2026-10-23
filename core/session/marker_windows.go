//go:build windows

package session

// marked leaves argv as it is: enowX doesn't measure processes on Windows.
func marked(_ string, argv []string) []string { return argv }
