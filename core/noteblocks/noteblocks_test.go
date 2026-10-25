package noteblocks

import (
	"strings"
	"testing"
)

const page = `# Plan

Intro

## Canvas

:::tabs
::tab Ringkasan
- [x] sketch
::tab Detail
### Inside
- [ ] build
:::

:::toggle More about it
hidden text
:::

## Tests
- [ ] run them
` + "```" + `
## not a heading
- [ ] not a to-do
` + "```" + `

::progress
`

func TestParseReadsLayoutBlocks(t *testing.T) {
	nodes := Parse(page)
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	if got := strings.Join(names, ","); got != ",tabs,,toggle,,progress" {
		t.Fatalf("nodes %q", got)
	}
	tabs := nodes[1]
	if len(tabs.Parts) != 2 || tabs.Parts[0].Title != "Ringkasan" || tabs.Parts[1].Title != "Detail" {
		t.Fatalf("tabs %+v", tabs.Parts)
	}
	if tabs.Start != 6 || tabs.End != 13 {
		t.Errorf("tabs on lines %d-%d", tabs.Start, tabs.End)
	}
	if tg := nodes[3]; tg.Args != "More about it" || len(tg.Parts) != 1 || tg.Parts[0].Nodes[0].Lines[0] != "hidden text" {
		t.Errorf("toggle %+v", tg)
	}
	// Nested and unclosed blocks.
	nested := Parse(":::columns\n::col\n:::callout info Note\nhi\n:::\n::col\nright")
	if len(nested) != 1 || len(nested[0].Parts) != 2 || nested[0].Parts[0].Nodes[0].Name != "callout" || nested[0].End != 7 {
		t.Errorf("nested %+v", nested)
	}
}

func TestOutlineAndSections(t *testing.T) {
	out := Outline(page)
	var got []string
	for _, s := range out {
		got = append(got, s.ID+":"+s.Kind)
	}
	want := "plan:h1,canvas:h2,ringkasan:tab,detail:tab,inside:h3,more-about-it:toggle,tests:h2"
	if strings.Join(got, ",") != want {
		t.Fatalf("outline %s\nwant    %s", strings.Join(got, ","), want)
	}
	canvas, err := Find(out, "canvas")
	if err != nil {
		t.Fatal(err)
	}
	if canvas.Done != 1 || canvas.Todos != 2 {
		t.Errorf("canvas progress %d/%d", canvas.Done, canvas.Todos)
	}
	if tests, _ := Find(out, "#tests"); tests.Todos != 1 {
		t.Errorf("tests counts %d to-dos, the one in code too?", tests.Todos)
	}
	if s, err := Find(out, "about"); err != nil || s.ID != "more-about-it" {
		t.Errorf("a part of a title: %v %v", s, err)
	}
	if _, err := Find(Outline("## A\n## A b\n"), "a"); err != nil {
		t.Errorf("a title exact wins: %v", err)
	}
	if _, err := Find(Outline("## Ab\n## Ac\n"), "a"); err == nil {
		t.Error("an ambiguous name found one")
	}

	if got := Text(page, canvas); !strings.HasPrefix(got, "## Canvas\n") || !strings.HasSuffix(got, ":::\n\n") || strings.Contains(got, "Tests") {
		t.Errorf("canvas text %q", got)
	}
	changed := Replace(page, canvas, "Just this.")
	if !strings.Contains(changed, "## Canvas\nJust this.\n\n## Tests\n") || strings.Contains(changed, "Ringkasan") {
		t.Errorf("replaced:\n%s", changed)
	}
	detail, _ := Find(out, "Detail")
	changed = Replace(page, detail, "new detail")
	if !strings.Contains(changed, "::tab Detail\nnew detail\n:::\n") {
		t.Errorf("a tab replaced:\n%s", changed)
	}
}

func TestCheckTicksOneTodo(t *testing.T) {
	md, line, err := Check(page, "build", true)
	if err != nil || line != 11 || !strings.Contains(md, "- [x] build") {
		t.Fatalf("check build: line %d %v\n%s", line, err, md)
	}
	if _, _, err := Check(page, "not a to-do", true); err == nil {
		t.Error("a to-do in code was ticked")
	}
	md, _, err = Check(md, "BUILD", false)
	if err != nil || !strings.Contains(md, "- [ ] build") {
		t.Errorf("uncheck: %v", err)
	}
	if _, _, err := Check("- [ ] a one\n- [ ] a two\n", "a", true); err == nil || !strings.Contains(err.Error(), "2 to-dos") {
		t.Errorf("ambiguous: %v", err)
	}
	if done, total := Progress(page); done != 1 || total != 3 {
		t.Errorf("progress %d/%d", done, total)
	}
}
