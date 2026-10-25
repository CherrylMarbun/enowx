package notesview

import "testing"

func TestBlocksKeepTheirMarkdown(t *testing.T) {
	md := "# Plan\n\nSome **bold** text\n- one\n  - nested\n1. first\n2. second\n- [ ] todo\n- [x] done\n> quoted\n---\n```go\nfmt.Println(1)\n\n```\n| a | b |\n|---|---|\n| 1 | 2 |\n## End\n"
	bs := parseBlocks(md)
	kinds := []kind{kH1, kText, kText, kBullet, kBullet, kNumber, kNumber, kTodo, kTodo, kQuote, kDivider, kCode, kTable, kH2}
	if len(bs) != len(kinds) {
		t.Fatalf("%d blocks, want %d: %+v", len(bs), len(kinds), bs)
	}
	for i, k := range kinds {
		if bs[i].kind != k {
			t.Errorf("block %d is %d, want %d: %+v", i, bs[i].kind, k, bs[i])
		}
	}
	if bs[4].indent != 1 || !bs[8].done || bs[7].done || bs[11].lang != "go" || bs[11].text != "fmt.Println(1)\n" {
		t.Errorf("details: %+v", bs)
	}
	if got := renderBlocks(bs); got != md {
		t.Errorf("written back:\n%q\nwant\n%q", got, md)
	}
	// Numbers follow the list, not what was typed.
	if got := renderBlocks(parseBlocks("3. a\n7. b\n\n1. c")); got != "1. a\n2. b\n\n1. c\n" {
		t.Errorf("numbers: %q", got)
	}
	if got := renderBlocks([]block{{}}); got != "" {
		t.Errorf("an empty page is %q", got)
	}
}

func TestTypedShortcutsMakeBlocks(t *testing.T) {
	for typed, want := range map[string]block{
		"## Title": {kind: kH2, text: "Title"},
		"[] call":  {kind: kTodo, text: "call"},
		"- item":   {kind: kBullet, text: "item"},
		"> said":   {kind: kQuote, text: "said"},
		"plain":    {kind: kText, text: "plain"},
	} {
		b := block{text: typed}
		b.typedShortcut()
		if b != want {
			t.Errorf("%q made %+v, want %+v", typed, b, want)
		}
	}
}
