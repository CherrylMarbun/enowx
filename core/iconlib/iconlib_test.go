package iconlib

import (
	"strings"
	"testing"
)

func TestSearchFindsByNameThenKeyword(t *testing.T) {
	got := Search("rocket", []string{"lucide"}, 5)
	if len(got) == 0 || got[0].Ref() != "lucide:rocket" {
		t.Fatalf("Search(rocket) = %v", got)
	}
	// "launch" is a keyword of Lucide's rocket, not a name.
	found := false
	for _, i := range Search("launch", []string{"lucide"}, 50) {
		found = found || i.Name == "rocket"
	}
	if !found {
		t.Error("rocket not found by its keyword launch")
	}
	for _, lib := range Libraries {
		if len(All(lib.ID)) < 1000 {
			t.Errorf("%s has %d icons", lib.ID, len(All(lib.ID)))
		}
	}
}

func TestParseRefs(t *testing.T) {
	for in, want := range map[string]string{
		"lucide:rocket":        "lucide:rocket",
		"tabler:rocket#E5484D": "tabler:rocket#e5484d",
		" phosphor:rocket ":    "phosphor:rocket",
		"":                     "",
	} {
		got, err := Check(in)
		if err != nil || got != want {
			t.Errorf("Check(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for in, says := range map[string]string{
		"🚀":            "use lucide:rocket",
		"lucide:rockt": "near it: lucide:rocket",
		"fa:rocket":    "no library",
		"rocket":       "give lib:name",
	} {
		if _, err := Check(in); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("Check(%q) = %v, want it to say %q", in, err, says)
		}
	}
}

func TestEveryEmojiMapsToAnIcon(t *testing.T) {
	for e, name := range emojiIcons {
		if _, ok := Lookup("lucide", name); !ok {
			t.Errorf("%s maps to %q, which Lucide hasn't", e, name)
		}
	}
	if ref, ok := FromEmoji("⚙️"); !ok || ref != "lucide:settings" {
		t.Errorf("FromEmoji(⚙️) = %s, %v", ref, ok)
	}
	if ref, ok := FromEmoji("🦄"); ok || ref != Default {
		t.Errorf("FromEmoji(🦄) = %s, %v; want the default, unknown", ref, ok)
	}
	if e, rest := LeadingEmoji("📌 TODO ENOWX"); e != "📌" || rest != "TODO ENOWX" {
		t.Errorf("LeadingEmoji = %q %q", e, rest)
	}
	if e, rest := LeadingEmoji("Plan 🚀"); e != "" || rest != "Plan 🚀" {
		t.Errorf("LeadingEmoji = %q %q", e, rest)
	}
}

func TestExportMarkdownDrawsIcons(t *testing.T) {
	md := "- [x] shipped :icon[lucide:check] :icon[nope:x]\n```svg\n<svg viewBox=\"0 0 10 10\"><script/><rect width=\"10\" height=\"10\"/></svg>\n```\nend"
	out := ExportMarkdown(md)
	if !strings.Contains(out, `<img alt="check" src="data:image/svg+xml;base64,`) || !strings.Contains(out, ":icon[nope:x]") {
		t.Errorf("icons: %s", out)
	}
	if !strings.Contains(out, "![drawing](data:image/svg+xml;base64,") || strings.Contains(out, "```") || !strings.HasSuffix(out, "\nend") {
		t.Errorf("drawing: %s", out)
	}
}

func TestSanitizeDropsWhatRunsOrLoads(t *testing.T) {
	in := `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="48px" height="32" onload="alert(1)">
<script>alert(2)</script>
<foreignObject><div>x</div></foreignObject>
<a href="https://evil.example"><path d="M0 0"/></a>
<image href="https://evil.example/x.png"/>
<use href="https://evil.example/s.svg#a"/><use href="#ok"/>
<path d="M1 1" fill="#ff0000" style="stroke:#00ff00;background:url(https://evil.example)" onclick="x()"/>
<rect fill="url(#grad)" width="3" height="3"/>
</svg>`
	out, err := Sanitize([]byte(in), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"script", "alert", "onload", "onclick", "foreignObject", "evil", "<a", "image", "#ff0000"} {
		if strings.Contains(out, bad) {
			t.Errorf("kept %q: %s", bad, out)
		}
	}
	for _, good := range []string{`viewBox="0 0 48 32"`, `width="48"`, `href="#ok"`, `fill="currentColor"`, `fill="url(#grad)"`} {
		if !strings.Contains(out, good) {
			t.Errorf("lost %q: %s", good, out)
		}
	}
	if _, err := Sanitize([]byte("<html><body/></html>"), false); err == nil {
		t.Error("not an SVG, cleaned")
	}
	if _, err := Sanitize([]byte("<svg>"+strings.Repeat(" ", MaxSVG)+"</svg>"), false); err == nil {
		t.Error("too big, cleaned")
	}
	ref, err := Custom([]byte(in), false)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(ref + "#112233")
	if err != nil || r.Custom == "" || r.Color != "#112233" || strings.Contains(r.SVG(), "script") {
		t.Errorf("Parse(custom) = %+v, %v", r, err)
	}
}
