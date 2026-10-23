package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestInstallLinkPointsAtTheApp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no links")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	exe := filepath.Join(t.TempDir(), "enowX")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link, err := InstallLink(exe)
	if err != nil || !Linked(exe) {
		t.Fatalf("link %s, %v", link, err)
	}
	// An app moved: the link follows it.
	moved := exe + "-2"
	if _, err := InstallLink(moved); err != nil || !Linked(moved) {
		t.Fatalf("relinked: %v", err)
	}
	// A file of the user's stays.
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte("mine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallLink(exe); err == nil {
		t.Fatal("replaced the user's own enowx")
	}
}
