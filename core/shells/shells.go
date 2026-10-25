// Package shells finds the shells a host has, for its terminals: the one
// its user logs in with and the others installed, and how each starts as
// a terminal's.
package shells

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Available is the host's shells, its user's login shell first.
func Available() []proto.ShellInfo {
	var out []proto.ShellInfo
	seen := map[string]bool{}
	add := func(path, name string) {
		if path == "" {
			return
		}
		key := path
		if real, err := filepath.EvalSymlinks(path); err == nil {
			key = real
		}
		key = strings.ToLower(key)
		if seen[key] {
			return
		}
		if st, err := os.Stat(path); err != nil || st.IsDir() {
			return
		}
		seen[key] = true
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(path), ".exe")
		}
		out = append(out, proto.ShellInfo{Name: name, Path: path})
	}
	add(Login(), "")
	for _, s := range candidates() {
		add(s.Path, s.Name)
	}
	if len(out) > 0 {
		out[0].Login = true
	}
	return out
}

// Find is the shell at path among those the host has.
func Find(path string) (proto.ShellInfo, bool) {
	for _, s := range Available() {
		if s.Path == path {
			return s, true
		}
	}
	return proto.ShellInfo{}, false
}
