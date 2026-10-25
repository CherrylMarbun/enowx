package widget

import (
	"strings"
	"testing"
)

func TestMarkdownBlocks(t *testing.T) {
	src := strings.Join([]string{
		"## Done", "", "Moved **all** handlers.", "",
		"| Pkg | Count |", "|---|---:|", "| anime | 9 |", "",
		"- one", "  - nested", "- [x] tested", "",
		"> careful", "---", "```go", "func f() {}", "```",
	}, "\n")
	var kinds []string
	for _, b := range parseBlocks(strings.Split(src, "\n")) {
		kinds = append(kinds, b.kind)
		switch b.kind {
		case "table":
			if len(b.rows) != 2 || b.rows[1][0] != "anime" {
				t.Fatalf("table = %v", b.rows)
			}
		case "list":
			if len(b.items) != 3 || b.items[1].depth != 1 || !b.items[2].done {
				t.Fatalf("list = %+v", b.items)
			}
		case "code":
			if b.lang != "go" || b.text != "func f() {}" {
				t.Fatalf("code = %+v", b)
			}
		}
	}
	if got := strings.Join(kinds, " "); got != "h p table list quote rule code" {
		t.Fatalf("blocks = %s", got)
	}
}

func TestInlineSpans(t *testing.T) {
	spans := inlineSpans("Run `go test` on **auth** and *this*, see [docs](https://x.dev) or https://y.dev. snake_case_name stays")
	var got []string
	for _, s := range spans {
		switch {
		case s.url != "":
			got = append(got, "link:"+s.text)
		case s.code:
			got = append(got, "code:"+s.text)
		case s.bold:
			got = append(got, "bold:"+s.text)
		case s.italic:
			got = append(got, "italic:"+s.text)
		}
	}
	want := "code:go test bold:auth italic:this link:docs link:https://y.dev"
	if strings.Join(got, " ") != want {
		t.Fatalf("spans = %v", got)
	}
	if last := spans[len(spans)-1]; !strings.Contains(last.text, "snake_case_name") {
		t.Fatalf("snake_case was styled: %+v", spans)
	}
}

// A note's own marks: links to pages and sections, badges; elsewhere
// they are text.
func TestNoteSpans(t *testing.T) {
	spans := noteSpans("See [[Release plan]], [[#Canvas]], [[TODO#Sync|the sync part]] {{High}} **[[Bold]]**", true)
	var got []string
	for _, s := range spans {
		switch {
		case s.url != "":
			got = append(got, s.url+"="+s.text)
		case s.badge:
			got = append(got, "badge:"+s.text)
		}
	}
	want := "note:Release plan=Release plan note:#Canvas=Canvas note:TODO#Sync|the sync part=the sync part badge:High note:Bold=Bold"
	if strings.Join(got, " ") != want {
		t.Fatalf("spans = %q", got)
	}
	if spans := inlineSpans("[[x]] {{y}}"); len(spans) != 1 || spans[0].url != "" {
		t.Fatalf("outside notes: %+v", spans)
	}
}
