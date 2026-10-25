package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// daemonExe is the program this machine's daemon runs as: on Windows a copy
// of the app's own, in dir. The daemon outlives the app's window, and
// Windows locks a running program's file, so the installer and updates
// couldn't replace the app's while it ran ("Error opening file for
// writing"). A copy per build: a daemon still running an older one keeps
// it until it is replaced, and copies no daemon runs any more go.
func daemonExe(exe, dir string) string {
	sum, err := fileSum(exe)
	if err != nil {
		log.Printf("daemon: %v; it runs as the app", err)
		return exe
	}
	copies := filepath.Join(dir, "daemon")
	want := filepath.Join(copies, "enowx-daemon-"+sum[:12]+".exe")
	if got, err := fileSum(want); err != nil || got != sum {
		if err := copyFile(exe, want); err != nil {
			log.Printf("daemon: copying the app: %v; it runs as the app", err)
			return exe
		}
	}
	// Older copies: removing one a daemon still runs fails, and is left.
	if old, err := filepath.Glob(filepath.Join(copies, "enowx-daemon-*.exe")); err == nil {
		for _, p := range old {
			if p != want && !strings.HasSuffix(p, ".tmp") {
				os.Remove(p)
			}
		}
	}
	return want
}

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
	return os.Rename(tmp, dst)
}
