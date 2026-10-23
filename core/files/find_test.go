package files

import (
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindRanksNamesFirstAndSkipsWhatGitIgnores(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"ui/chatview/composer.go", "ui/chatview/chatview.go", "docs/chat-view.md",
		"core/session/session.go", "node_modules/chat/index.js", "README.md",
	} {
		write(t, filepath.Join(root, p), "x")
	}
	write(t, filepath.Join(root, ".gitignore"), "node_modules/\n")
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	got, err := Find(root, "chatview", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[0] != "ui/chatview/chatview.go" {
		t.Fatalf("chatview → %q", got)
	}
	got, _ = Find(root, "cmpsr", 10)
	if !slices.Equal(got, []string{"ui/chatview/composer.go"}) {
		t.Fatalf("cmpsr → %q", got)
	}
	got, _ = Find(root, "index", 10)
	if len(got) != 0 {
		t.Fatalf("an ignored file was found: %q", got)
	}
	// Outside git, folders of dependencies are left out.
	plain := t.TempDir()
	write(t, filepath.Join(plain, "src/main.go"), "x")
	write(t, filepath.Join(plain, "node_modules/x/main.js"), "x")
	got, _ = Find(plain, "main", 10)
	if !slices.Equal(got, []string{"src/main.go"}) {
		t.Fatalf("plain folder → %q", got)
	}
}
