package projectlinks

import (
	"path"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Match reports whether the file at p (with slashes, relative to the
// project) matches pattern: a pattern without a slash matches a file or
// folder of that name anywhere ("node_modules", "*.log"); with one, the
// path from the project's folder, where * stays within a folder and **
// spans any ("src/**/*.test.ts"), a folder matching taking what is
// inside.
func Match(pattern, p string) bool {
	pattern = strings.TrimPrefix(strings.TrimSuffix(pattern, "/"), "/")
	if pattern == "" {
		return false
	}
	parts := strings.Split(p, "/")
	if !strings.Contains(pattern, "/") {
		for _, name := range parts {
			if ok, _ := path.Match(pattern, name); ok {
				return true
			}
		}
		return false
	}
	pp := strings.Split(pattern, "/")
	for n := len(parts); n > 0; n-- { // the file, or a folder holding it
		if matchParts(pp, parts[:n]) {
			return true
		}
	}
	return false
}

func matchParts(pattern, parts []string) bool {
	if len(pattern) == 0 {
		return len(parts) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(parts); i++ {
			if matchParts(pattern[1:], parts[i:]) {
				return true
			}
		}
		return false
	}
	if len(parts) == 0 {
		return false
	}
	if ok, _ := path.Match(pattern[0], parts[0]); !ok {
		return false
	}
	return matchParts(pattern[1:], parts[1:])
}

// Synced reports whether a link syncs the file at p: not enowX's own
// folder, among include's when there are any, and none of exclude's.
func Synced(p string, include, exclude []string) bool {
	if p == LocalDir || strings.HasPrefix(p, LocalDir+"/") {
		return false
	}
	for _, x := range exclude {
		if Match(x, p) {
			return false
		}
	}
	if len(include) == 0 {
		return true
	}
	for _, x := range include {
		if Match(x, p) {
			return true
		}
	}
	return false
}

// Filter keeps the files a link syncs.
func Filter(files []proto.SyncFile, include, exclude []string) []proto.SyncFile {
	out := files[:0:0]
	for _, f := range files {
		if Synced(f.Path, include, exclude) {
			out = append(out, f)
		}
	}
	return out
}
