// Package usersync keeps the user's data the same on every host: notes,
// canvases, settings, the vault's ciphertext, each a file under a
// daemon's folder. Two hosts sync file by file, three ways against what
// they had at their last sync: what changed on one side goes to the
// other, what changed on both merges (text by lines, notes' headers by
// field, lists by their items' names), and what doesn't merge is kept as
// a conflict for the user, never overwritten.
package usersync

import (
	"path"
	"strings"
)

// Roots are the folders synced, under a daemon's folder. A feature
// keeping the user's data in one of them is synced with no more to do.
var Roots = []string{"notes/", "canvas/", "vault/", "settings/", "catalog/", ConflictsDir}

// ConflictsDir holds the conflicts, synced so every host shows them.
const ConflictsDir = "sync/conflicts/"

// Kept per host, never synced: what an agent proposed (its session is
// here), versions kept for undo, files being written.
var excluded = []string{"notes/proposals.json", "notes/.history/"}

// Synced reports whether p, a slash path under a daemon's folder, is
// synced.
func Synced(p string) bool {
	if p == "" || path.IsAbs(p) || strings.Contains(p, "\\") || path.Clean(p) != p || strings.HasPrefix(p, "../") || p == ".." {
		return false
	}
	if strings.HasSuffix(p, ".tmp") || strings.HasPrefix(p, ".") || strings.Contains(p, "/.") {
		return false
	}
	for _, x := range excluded {
		if p == x || strings.HasSuffix(x, "/") && strings.HasPrefix(p, x) {
			return false
		}
	}
	for _, r := range Roots {
		if strings.HasPrefix(p, r) && len(p) > len(r) {
			return true
		}
	}
	return false
}
