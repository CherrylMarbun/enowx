package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseFoldersThroughTheProtocol(t *testing.T) {
	c := connect(t, startServer(t), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, dir := range []string{"code/app/.git", "code/notes", ".config"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "code", "README"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := c.ListFolders("~", false)
	if err != nil {
		t.Fatal(err)
	}
	if l.Path != home || len(l.Folders) != 1 || l.Folders[0].Name != "code" {
		t.Fatalf("home = %+v", l)
	}
	if l, _ = c.ListFolders("~", true); len(l.Folders) != 2 {
		t.Fatalf("with hidden folders: %+v", l.Folders)
	}
	l, err = c.ListFolders("~/code", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Folders) != 2 || !l.Folders[0].Git || l.Folders[1].Git || l.Files != 1 || l.Parent != home {
		t.Fatalf("code = %+v", l)
	}

	made, err := c.MakeFolder("~/code/new-app")
	if err != nil || made != filepath.Join(home, "code", "new-app") {
		t.Fatalf("make = %q, %v", made, err)
	}
	if _, err := c.MakeFolder("~/code/new-app"); err == nil {
		t.Fatal("made a folder that exists")
	}
	if _, err := c.ListFolders("~/missing", false); err == nil {
		t.Fatal("listed a missing folder")
	}
}
