//go:build !windows

package transport

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// A daemon that stops removes its socket file, but not one another daemon
// listens on now.
func TestClosingLeavesAnotherDaemonsSocket(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "enx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "d.sock")
	old, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	// Another daemon takes the path, as one started where the old one's
	// home was moved away would.
	os.Remove(path)
	cur, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer cur.Close()
	old.Close()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatalf("the serving daemon's socket went: %v", err)
	}
	c.Close()

	own, err := Listen(filepath.Join(dir, "own.sock"))
	if err != nil {
		t.Fatal(err)
	}
	own.Close()
	if _, err := os.Stat(filepath.Join(dir, "own.sock")); !os.IsNotExist(err) {
		t.Fatalf("its own socket stayed: %v", err)
	}
}
