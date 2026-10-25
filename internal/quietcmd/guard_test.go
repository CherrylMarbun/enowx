package quietcmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// allowed start programs directly on purpose: the daemon, started
// detached, which opens no console itself.
var allowed = map[string]bool{
	"daemon/run.go":        true,
	"hostconn/hostconn.go": true,
}

var direct = regexp.MustCompile(`\bexec\.Command(Context)?\(`)

// Code that runs on Windows starts programs through this package, so none
// opens a console window there: exec.Command is only for files built
// elsewhere only.
func TestNoConsoleWindowsOnWindows(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || rel == "site" || rel == "internal/quietcmd" || rel == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || allowed[rel] || notWindows(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), "//go:build !windows") || strings.Contains(string(data), "//go:build linux") || strings.Contains(string(data), "//go:build darwin") {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if direct.MatchString(line) {
				t.Errorf("%s:%d starts a program with os/exec, which opens a console window on Windows: use quietcmd", rel, i+1)
			}
		}
		return nil
	})
}

// notWindows reports whether a file's name keeps it off Windows.
func notWindows(rel string) bool {
	base := strings.TrimSuffix(filepath.Base(rel), ".go")
	for _, suffix := range []string{"_unix", "_linux", "_darwin", "_android", "_freebsd"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}
