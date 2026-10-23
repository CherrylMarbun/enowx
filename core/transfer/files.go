// Package transfer copies a project and its sessions from one host to
// another: the source's daemon packs them into an archive, the app carries
// it across in parts, and the target's daemon unpacks it and starts the
// sessions again, their agents on their conversations. See
// docs/transfer.md.
package transfer

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/core/git"
)

// Skipped are folders of dependencies, builds and caches, which the target
// installs or builds again; a project outside git leaves them out.
var Skipped = []string{
	"node_modules", ".venv", "venv", "__pycache__", "target", "dist", ".next", ".nuxt", ".turbo",
	".svelte-kit", ".parcel-cache", ".cache", ".gradle", "Pods", ".dart_tool", ".pytest_cache",
	".mypy_cache", ".tox", ".DS_Store",
}

// IsEnvFile reports whether name is a file of secrets, as .env and
// .env.local are: it goes only when the user says so.
func IsEnvFile(name string) bool {
	return name == ".env" || strings.HasPrefix(name, ".env.")
}

// ProjectFiles lists the files of the project in dir to carry, relative
// to it, with slashes: in a git repository, what git doesn't ignore (its
// tracked and new files), and its .git folder when it has one (a worktree
// has a file, which isn't carried); elsewhere every file but the Skipped
// folders'. env adds the .env files, which git usually ignores.
func ProjectFiles(dir string, env bool) ([]string, error) {
	var out []string
	if _, err := git.TopLevel(dir); err == nil {
		listed, err := git.Run(dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
		if err != nil {
			return nil, err
		}
		for _, f := range strings.Split(listed, "\x00") {
			if f == "" || !env && IsEnvFile(filepath.Base(f)) {
				continue
			}
			if info, err := os.Lstat(filepath.Join(dir, f)); err == nil && !info.IsDir() {
				out = append(out, f)
			}
		}
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			// Not its worktrees: they are the source's folders, and would
			// keep their branches from being checked out here.
			if err := walk(dir, ".git", nil, func(rel string) {
				if !strings.HasPrefix(rel, ".git/worktrees/") {
					out = append(out, rel)
				}
			}); err != nil {
				return nil, err
			}
		}
	} else if err := walk(dir, "", Skipped, func(rel string) {
		if env || !IsEnvFile(filepath.Base(rel)) {
			out = append(out, rel)
		}
	}); err != nil {
		return nil, err
	}
	if env {
		_ = walk(dir, "", append(slices.Clone(Skipped), ".git"), func(rel string) {
			if IsEnvFile(filepath.Base(rel)) && !slices.Contains(out, rel) {
				out = append(out, rel)
			}
		})
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// walk calls add for each file under dir/sub, skipping folders named in
// skip, with paths relative to dir.
func walk(dir, sub string, skip []string, add func(rel string)) error {
	root := filepath.Join(dir, sub)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil // unreadable: left behind
		}
		if d.IsDir() {
			if path != root && slices.Contains(skip, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err == nil {
			add(filepath.ToSlash(rel))
		}
		return nil
	})
}
