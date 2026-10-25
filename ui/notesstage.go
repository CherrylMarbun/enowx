package ui

import (
	"archive/zip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// stageLocal sends paths to the host with stage: one file as it is, a
// folder or several files as one .zip.
func stageLocal(stage func(path string) (string, error), paths []string) (string, error) {
	if len(paths) == 1 {
		if info, err := os.Stat(paths[0]); err == nil && !info.IsDir() {
			return stage(paths[0])
		}
	}
	// Named for what it holds, as the host's import says.
	dir, err := os.MkdirTemp("", "enowx-import-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	name := "Notes.zip"
	if len(paths) == 1 {
		name = filepath.Base(paths[0]) + ".zip"
	}
	tmp, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(tmp)
	for _, p := range paths {
		if err := zipInto(zw, p); err != nil {
			tmp.Close()
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return stage(tmp.Name())
}

// zipInto adds the file or folder at p to zw under its name.
func zipInto(zw *zip.Writer, p string) error {
	base := filepath.Dir(p)
	return filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != p && strings.HasPrefix(d.Name(), ".") && d.Name() != ".obsidian" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
}
