package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDaemonCopyPreservesBuildIdentity(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "enowX.exe")
	if err := os.WriteFile(exe, []byte("same executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1700000000, 123456700)
	if err := os.Chtimes(exe, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	copied, err := daemonCopy(exe, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourceInfo, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	copyInfo, err := os.Stat(copied)
	if err != nil {
		t.Fatal(err)
	}
	if sourceInfo.Size() != copyInfo.Size() || !sourceInfo.ModTime().Equal(copyInfo.ModTime()) {
		t.Fatalf("identical executables have different build identities: source=%d/%s copy=%d/%s",
			sourceInfo.Size(), sourceInfo.ModTime().UTC().Format(time.RFC3339Nano),
			copyInfo.Size(), copyInfo.ModTime().UTC().Format(time.RFC3339Nano))
	}
}
