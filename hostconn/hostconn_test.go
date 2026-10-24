package hostconn

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/transport"
)

func TestParseInspect(t *testing.T) {
	uname, v, h := parseInspect("Linux x86_64\nv:0.0.15\n\nh:abc123\n\n")
	if uname != "Linux x86_64" || v != "0.0.15" || h != "abc123" {
		t.Fatalf("%q %q %q", uname, v, h)
	}
	// No daemon installed, no hash tool.
	uname, v, h = parseInspect("Linux aarch64\nv:\nh:\n")
	if uname != "Linux aarch64" || v != "" || h != "" {
		t.Fatalf("%q %q %q", uname, v, h)
	}
}

func TestInspectScriptReadsVersionAndHash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	home := t.TempDir()
	bin := filepath.Join(home, transport.RemoteDaemonPath)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 9.9.9\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := inspectScript(`"$HOME/` + transport.RemoteDaemonPath + `"`)
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	_, v, h := parseInspect(string(out))
	if v != "9.9.9" || len(h) != 64 || strings.Trim(h, "0123456789abcdef") != "" {
		t.Fatalf("version %q hash %q from %q", v, h, out)
	}
}

func TestRunsOther(t *testing.T) {
	for _, c := range []struct {
		have, want string
		other      bool
	}{
		{"a", "a", false},
		{"a", "b", true},
		{"", "b", true},  // a daemon older than hashes
		{"a", "", false}, // no embedded daemon for this server
	} {
		if got := runsOther(c.have, c.want); got != c.other {
			t.Errorf("runsOther(%q, %q) = %v, want %v", c.have, c.want, got, c.other)
		}
	}
}
