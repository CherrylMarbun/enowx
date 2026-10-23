package commands

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestCommandsOfAProject(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"scripts":{"zip":"x","build":"vite build","dev":"vite"}}`)
	write("bun.lock", "")
	write("Makefile", ".PHONY: up\nup:\n\tdocker compose up\n")
	s, err := OpenStore(filepath.Join(t.TempDir(), "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	names := func() []string {
		var out []string
		for _, c := range s.For("app", dir) {
			out = append(out, c.Source+":"+c.Command)
		}
		return out
	}
	want := []string{"package.json:bun run dev", "package.json:bun run build", "package.json:bun run zip", "Makefile:make up"}
	if got := names(); !slices.Equal(got, want) {
		t.Fatalf("found %q, want %q", got, want)
	}
	// Running one puts it first; saving one puts it before, once.
	if err := s.Ran("app", proto.RunCommand{Command: "bun run build"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("app", proto.RunCommand{Name: "tunnel", Command: "cloudflared tunnel run"}); err != nil {
		t.Fatal(err)
	}
	want = []string{"saved:cloudflared tunnel run", "recent:bun run build", "package.json:bun run dev", "package.json:bun run zip", "Makefile:make up"}
	if got := names(); !slices.Equal(got, want) {
		t.Fatalf("after running and saving %q, want %q", got, want)
	}
	// The store reads back what it saved.
	again, err := OpenStore(s.path)
	if err != nil || len(again.For("app", dir)) != len(want) {
		t.Fatalf("reopened: %v, %v", again.For("app", dir), err)
	}
}
