package unslop

import (
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// A step's changes: an element's own, not those it inherits; what came
// and went.
func TestChanges(t *testing.T) {
	before := `{"page":{"p":"","x":0,"y":0,"w":800,"h":600,"font":"Arial","size":"16px","color":"rgb(0, 0, 0)","pad":"13px"},
		"title":{"p":"page","x":10,"y":10,"w":300,"h":40,"font":"Arial","size":"34px","weight":"700","color":"rgb(0, 0, 0)"},
		"badge":{"p":"page","x":0,"y":0,"w":40,"h":20}}`
	after := `{"page":{"p":"","x":0,"y":0,"w":800,"h":600,"font":"Geist","size":"16px","color":"rgb(0, 0, 0)","pad":"16px"},
		"title":{"p":"page","x":10,"y":12,"w":300,"h":44,"font":"Geist","size":"36px","weight":"300","color":"rgb(0, 0, 0)"},
		"cta":{"p":"page","x":10,"y":80,"w":120,"h":40}}`
	got := Changes(before, after)
	if len(got) != 4 {
		t.Fatalf("changes: %+v", got)
	}
	by := map[string]proto.UnslopChange{}
	for _, c := range got {
		by[c.Element] = c
	}
	title, page := by["title"], by["page"]
	if title.Element != "title" || len(title.Props) != 2 || title.Props[0].Name != "size" || title.Props[0].From != "34px" || title.Props[0].To != "36px" {
		t.Fatalf("title: %+v", title)
	}
	if page.Element != "page" || len(page.Props) != 2 {
		t.Fatalf("page: %+v", page)
	}
	if !by["cta"].Added || !by["badge"].Removed {
		t.Fatalf("came and went: %+v", got)
	}
}

// A story of many steps plays as a few, crops on what changed most, and
// carries each design.
func TestStory(t *testing.T) {
	st := proto.UnslopStory{Width: 1200, Height: 800, From: 20, Score: 85, Docs: []string{"<p>0</p>"}}
	for i := range 12 {
		st.Docs = append(st.Docs, "<p>"+string(rune('a'+i))+"</p>")
		step := proto.UnslopStep{Passes: []string{Names()[i]}}
		if i == 3 {
			step.Changes = []proto.UnslopChange{{Element: "t", X: 100, Y: 200, W: 300, H: 50}}
		}
		st.Steps = append(st.Steps, step)
	}
	g := StoryGroup(st, 4)
	if len(g.Steps) != 4 || len(g.Docs) != 5 || g.Docs[4] != st.Docs[12] || len(g.Steps[1].Passes) != 3 {
		t.Fatalf("grouped: %d steps, %d docs", len(g.Steps), len(g.Docs))
	}
	if x, y, w := Crop(st); w != 400 || x != 50 || y != 25 {
		t.Fatalf("crop %d %d %d", x, y, w)
	}
	page := StoryHTML(g, "")
	if !strings.Contains(page, `id="d4"`) || !strings.Contains(page, "Unslopped in 4 steps · 20 → 85") {
		t.Fatal("story page")
	}
}
