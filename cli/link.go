package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// LinkPath is where InstallLink puts the enowx command: ~/.local/bin, which
// login shells put on PATH when it exists.
func LinkPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin", "enowx"), nil
}

// Linked reports whether the enowx command is installed, pointing at exe.
func Linked(exe string) bool {
	link, err := LinkPath()
	if err != nil {
		return false
	}
	target, err := os.Readlink(link)
	return err == nil && target == exe
}

// InstallLink makes the enowx command a link to exe, the app's binary,
// replacing an older link but never a file of the user's.
func InstallLink(exe string) (string, error) {
	if runtime.GOOS == "windows" {
		return "", errors.New("cli: on Windows, add the app's folder to PATH to run enowx")
	}
	link, err := LinkPath()
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("cli: %s exists and isn't enowX's link: move it away first", link)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return "", err
	}
	_ = os.Remove(link)
	if err := os.Symlink(exe, link); err != nil {
		return "", fmt.Errorf("cli: link %s: %w", link, err)
	}
	return link, nil
}

// OnPath reports whether the folder of the enowx command is on PATH.
func OnPath() bool {
	link, err := LinkPath()
	if err != nil {
		return false
	}
	return slices.Contains(filepath.SplitList(os.Getenv("PATH")), filepath.Dir(link))
}
