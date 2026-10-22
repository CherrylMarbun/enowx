// Package folders lists and creates folders, so the app can browse any
// host's disk through its daemon when picking a project.
package folders

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Resolve makes p absolute: "" and "~" are home, "~/x" is under it, and a
// relative path is taken from home.
func Resolve(home, p string) string {
	p = strings.TrimSpace(p)
	switch {
	case p == "" || p == "~":
		return filepath.Clean(home)
	case strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`):
		return filepath.Join(home, p[2:])
	case filepath.IsAbs(p):
		return filepath.Clean(p)
	}
	return filepath.Join(home, p)
}

// List returns the folders in p, sorted by name, links to folders
// included.
func List(home string, params proto.FolderListParams) (proto.FolderListing, error) {
	dir := Resolve(home, params.Path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return proto.FolderListing{}, fmt.Errorf("folders: %w", err)
	}
	out := proto.FolderListing{Path: dir, Home: home, Folders: []proto.FolderEntry{}, Git: inGit(dir)}
	if parent := filepath.Dir(dir); parent != dir {
		out.Parent = parent
	}
	for _, e := range entries {
		name := e.Name()
		if !params.Hidden && strings.HasPrefix(name, ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(filepath.Join(dir, name))
			isDir = err == nil && info.IsDir()
		}
		if !isDir {
			out.Files++
			continue
		}
		out.Folders = append(out.Folders, proto.FolderEntry{Name: name, Git: exists(filepath.Join(dir, name, ".git"))})
	}
	slices.SortFunc(out.Folders, func(a, b proto.FolderEntry) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

// Make creates the folder p, and its parents.
func Make(home, p string) (string, error) {
	dir := Resolve(home, p)
	if name := filepath.Base(dir); name == "." || name == ".." || strings.TrimSpace(name) == "" {
		return "", errors.New("folders: give the new folder a name")
	}
	if exists(dir) {
		return "", fmt.Errorf("folders: %s already exists", filepath.Base(dir))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("folders: %w", err)
	}
	return dir, nil
}

// inGit reports whether dir is inside a git repository.
func inGit(dir string) bool {
	for {
		if exists(filepath.Join(dir, ".git")) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
