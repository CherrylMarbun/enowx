//go:build windows

package transport

import "os"

// private reports whether dir is a folder; the temporary folder it is in
// is the user's own on Windows.
func private(dir string) bool {
	fi, err := os.Stat(dir)
	return err == nil && fi.IsDir()
}
