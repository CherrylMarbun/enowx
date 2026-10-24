package files

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/enowx/enowx/proto"
)

// Change makes change c in root: a new file or folder, a rename, a copy,
// a delete. It never touches root itself, nor anything outside it, and
// never replaces what is there.
func Change(root string, c proto.FileChange) (string, error) {
	abs, rel, err := resolve(root, c.Path)
	if err != nil {
		return "", err
	}
	named := func() (string, string, error) { // the new name, in folder dir
		name := strings.TrimSpace(c.Name)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
			return "", "", fmt.Errorf("files: %q isn't a name: no slashes", c.Name)
		}
		return name, path.Join(rel, name), nil
	}
	switch c.Change {
	case proto.FileNewFile, proto.FileNewFolder:
		_, to, err := named()
		if err != nil {
			return "", err
		}
		target := filepath.Join(abs, filepath.Base(to))
		if c.Change == proto.FileNewFolder {
			if err := os.Mkdir(target, 0o755); err != nil {
				return "", fmt.Errorf("files: %w", err)
			}
			return to, nil
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return "", fmt.Errorf("files: %w", err)
		}
		return to, f.Close()
	}
	if rel == "" {
		return "", errors.New("files: that is the session's folder itself")
	}
	if _, err := os.Lstat(abs); err != nil {
		return "", fmt.Errorf("files: %w", err)
	}
	switch c.Change {
	case proto.FileRename:
		name, _, err := named()
		if err != nil {
			return "", err
		}
		to := path.Join(path.Dir(rel), name)
		target := filepath.Join(filepath.Dir(abs), name)
		if _, err := os.Lstat(target); err == nil {
			return "", fmt.Errorf("files: %s exists already", name)
		}
		if err := os.Rename(abs, target); err != nil {
			return "", fmt.Errorf("files: %w", err)
		}
		return strings.TrimPrefix(to, "./"), nil
	case proto.FileDuplicate:
		target := copyName(abs)
		if err := copyTree(abs, target); err != nil {
			return "", fmt.Errorf("files: %w", err)
		}
		return strings.TrimPrefix(path.Join(path.Dir(rel), filepath.Base(target)), "./"), nil
	case proto.FileDelete:
		if err := os.RemoveAll(abs); err != nil {
			return "", fmt.Errorf("files: %w", err)
		}
		return "", nil
	}
	return "", fmt.Errorf("files: no change %q", c.Change)
}

// copyName is a free name beside abs for its copy: "a copy.txt", "a copy
// 2.txt".
func copyName(abs string) string {
	dir, base := filepath.Split(abs)
	ext := filepath.Ext(base)
	if strings.HasPrefix(base, ".") && strings.Count(base, ".") == 1 {
		ext = "" // a dotfile's name is all name
	}
	stem := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		name := stem + " copy" + ext
		if i > 1 {
			name = fmt.Sprintf("%s copy %d%s", stem, i, ext)
		}
		p := filepath.Join(dir, name)
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p
		}
	}
}

// copyTree copies a file, link or folder from to to, keeping modes.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, dst)
		case d.IsDir():
			return os.Mkdir(dst, info.Mode().Perm())
		}
		src, err := os.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

// statLimit bounds how many entries a folder's size counts.
const statLimit = 50000

// Stat says what rel of root is; a folder's size and count are of what
// it holds, the first statLimit entries.
func Stat(root, rel string) (proto.FileStat, error) {
	abs, clean, err := resolve(root, rel)
	if err != nil {
		return proto.FileStat{}, err
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return proto.FileStat{}, fmt.Errorf("files: %w", err)
	}
	st := proto.FileStat{Path: clean, Abs: abs, Size: info.Size(), Modified: info.ModTime(), Mode: info.Mode().String(), Owner: owner(info)}
	if info.Mode()&os.ModeSymlink != 0 {
		st.Link, _ = os.Readlink(abs)
		if t, err := os.Stat(abs); err == nil {
			info = t
		}
	}
	if info.IsDir() {
		st.Dir, st.Size = true, 0
		n := 0
		_ = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err != nil || p == abs {
				return nil
			}
			if n++; n > statLimit {
				st.Partial = true
				return filepath.SkipAll
			}
			if d.IsDir() {
				st.Folders++
				return nil
			}
			st.Files++
			if i, err := d.Info(); err == nil {
				st.Size += i.Size()
			}
			return nil
		})
	}
	if parent := path.Dir(clean); clean != "" {
		if parent == "." {
			parent = ""
		}
		entries := []proto.FileEntry{{Name: path.Base(clean), Dir: st.Dir}}
		markGit(root, parent, entries)
		st.Status, st.Ignored = entries[0].Status, entries[0].Ignored
	}
	return st, nil
}
