package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// daemonCopy copies the app's executable into dir for the daemon to run
// as, with the libraries beside it (the terminal emulator's ghostty-vt.dll,
// which it loads from next to itself), and returns the copy. A folder per
// build: a daemon still running an older one keeps it until it is
// replaced, and folders no daemon runs any more go.
func daemonCopy(exe, dir string) (string, error) {
	sum, err := fileSum(exe)
	if err != nil {
		return "", err
	}
	copies := filepath.Join(dir, "daemon")
	build := filepath.Join(copies, sum[:12])
	want := filepath.Join(build, "enowx-daemon"+filepath.Ext(exe))
	files := map[string]string{exe: want}
	if dlls, err := filepath.Glob(filepath.Join(filepath.Dir(exe), "*.dll")); err == nil {
		for _, d := range dlls {
			files[d] = filepath.Join(build, filepath.Base(d))
		}
	}
	for src, dst := range files {
		if sameFile(src, dst) {
			continue
		}
		if err := copyFile(src, dst); err != nil {
			return "", fmt.Errorf("copying %s: %w", filepath.Base(src), err)
		}
	}
	// Older builds: removing one a daemon still runs fails, and is left.
	if old, err := os.ReadDir(copies); err == nil {
		for _, e := range old {
			if p := filepath.Join(copies, e.Name()); p != build {
				os.RemoveAll(p)
			}
		}
	}
	return want, nil
}

// sameFile says dst is already a copy of src: as big, and no older.
func sameFile(src, dst string) bool {
	a, err := os.Stat(src)
	if err != nil {
		return false
	}
	b, err := os.Stat(dst)
	return err == nil && a.Size() == b.Size() && !b.ModTime().Before(a.ModTime())
}

//lint:ignore U1000 used on Windows (daemonexe_windows.go), and by the tests
func fileSum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile copies src to dst whole or not at all.
//
//lint:ignore U1000 used on Windows (daemonexe_windows.go), and by the tests
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// version.Build includes the timestamp: identical copies must not
	// look like different builds and repeatedly replace each other.
	info, err := in.Stat()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chtimes(tmp, info.ModTime(), info.ModTime()); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
