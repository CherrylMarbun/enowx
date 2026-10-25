package projectctx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContextSumsUpAProject(t *testing.T) {
	dir := t.TempDir()
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(filepath.Join(dir, p)), 0o755)
		os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644)
	}
	write("go.mod", "module x\n")
	write("README.md", "# Antares\nA shop.\n")
	write("internal/auth/login.go", "package auth\n")
	write("internal/auth/token.go", "package auth\n")
	write("cmd/main.go", "package main\n")
	c, err := Of(dir)
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.Join(c.Tree, "\n")
	if c.Stack[0] != "Go" || !strings.Contains(c.Readme, "A shop.") || !strings.Contains(tree, "internal/auth/ (2 files)") || !strings.Contains(tree, "go.mod") {
		t.Fatalf("context %+v", c)
	}
	f, err := Read(dir, "internal/auth/login.go")
	if err != nil || f.Text != "package auth\n" {
		t.Fatalf("read %+v, %v", f, err)
	}
	if _, err := Read(dir, "../outside"); err == nil {
		t.Error("read a file out of the project")
	}
}
