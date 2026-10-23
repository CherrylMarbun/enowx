package transport

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A deep ENOWX_HOME still gets a daemon socket that listens and answers,
// one of its own.
func TestDeepHomeGetsAShortSocket(t *testing.T) {
	deep := filepath.Join(t.TempDir(), strings.Repeat("a-rather-long-folder-name/", 5))
	t.Setenv("ENOWX_HOME", deep)
	path, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(path)) })
	if len(path) > maxSocketPath {
		t.Fatalf("socket path %d bytes: %s", len(path), path)
	}
	l, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		if c, err := l.Accept(); err == nil {
			c.Close()
		}
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	t.Setenv("ENOWX_HOME", deep+"other")
	other, err := SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(other)) })
	if other == path {
		t.Fatalf("two homes share %s", path)
	}
}
