package libraryview

import (
	"slices"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	got := splitCommand(`npx -y "@scope/server files" --root '~/my code'`)
	want := []string{"npx", "-y", "@scope/server files", "--root", "~/my code"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestFrontMatter(t *testing.T) {
	fields, body := frontMatter("---\nname: review\ndescription: \"Reviews code\"\n---\n\n# Review\n")
	if len(fields) != 2 || fields[1] != [2]string{"description", "Reviews code"} || body != "# Review\n" {
		t.Fatalf("fields %q, body %q", fields, body)
	}
	if fields, body := frontMatter("# No front matter"); fields != nil || body != "# No front matter" {
		t.Fatalf("plain: %q %q", fields, body)
	}
}

func TestFrontMatterBlocks(t *testing.T) {
	fields, _ := frontMatter("---\nname: x\ndescription: >\n  Folds these\n  lines.\nlicense: MIT\n---\nbody")
	if len(fields) != 3 || fields[1][1] != "Folds these lines." || fields[2][1] != "MIT" {
		t.Fatalf("fields %q", fields)
	}
}
