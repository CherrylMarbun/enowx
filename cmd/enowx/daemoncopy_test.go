package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The daemon's copy has the app's libraries beside it, as the terminal
// emulator's, which it loads from next to itself; another build's copy
// goes.
func TestDaemonCopyBringsTheLibraries(t *testing.T) {
	app := t.TempDir()
	exe := filepath.Join(app, "enowX.exe")
	os.WriteFile(exe, []byte("the app"), 0o755)
	os.WriteFile(filepath.Join(app, "ghostty-vt.dll"), []byte("the terminal"), 0o644)
	dir := t.TempDir()
	old := filepath.Join(dir, "daemon", "0123456789ab")
	os.MkdirAll(old, 0o755)

	got, err := daemonCopy(exe, dir)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(got); string(b) != "the app" {
		t.Fatalf("copy %s: %q", got, b)
	}
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(got), "ghostty-vt.dll")); err != nil || string(b) != "the terminal" {
		t.Fatalf("no library beside the copy: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("another build's copy stayed: %v", err)
	}
	if again, err := daemonCopy(exe, dir); err != nil || again != got {
		t.Fatalf("again: %s %v", again, err)
	}
}
