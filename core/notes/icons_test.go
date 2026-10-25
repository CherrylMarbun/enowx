package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// Notes and groups of before icons were SVG get theirs: the closest icon,
// else the default marked for review; a title's leading emoji becomes the
// icon; the text's emoji stay.
func TestMigrateIconsFromEmoji(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(id, header, body string) {
		data := "---\ntitle: " + header + "\norder: 0\ncreated: 2026-10-25T16:34:30Z\nupdated: 2026-10-25T16:34:30Z\n---\n\n" + body
		if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("aaaaaaaaaaaa", "TODO ENOWX\nicon: 📌", "- [ ] 🚀 ship\n")
	write("bbbbbbbbbbbb", "Unicorns\nicon: 🦄", "")
	write("cccccccccccc", "💡 Ideas", "")
	write("dddddddddddd", "Plain", "")
	if err := os.WriteFile(filepath.Join(dir, "groups.json"), []byte(`[{"name":"Work","icon":"⚙️"},{"name":"Fun","icon":"🦄"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Open(dir)
	n, err := s.MigrateIcons()
	if err != nil || n != 5 {
		t.Fatalf("MigrateIcons = %d, %v; want 5 changed", n, err)
	}
	want := map[string][3]string{
		"aaaaaaaaaaaa": {"TODO ENOWX", "lucide:pin", "false"},
		"bbbbbbbbbbbb": {"Unicorns", "lucide:file-text", "true"},
		"cccccccccccc": {"Ideas", "lucide:lightbulb", "false"},
		"dddddddddddd": {"Plain", "", "false"},
	}
	for id, w := range want {
		got, err := s.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != w[0] || got.Icon != w[1] || (got.IconReview != (w[2] == "true")) {
			t.Errorf("%s = %q %q review %v; want %v", id, got.Title, got.Icon, got.IconReview, w)
		}
	}
	if got, _ := s.Get("aaaaaaaaaaaa"); got.Body != "- [ ] 🚀 ship\n" {
		t.Errorf("text changed: %q", got.Body)
	}
	groups, _ := s.Groups()
	if groups[0].Icon != "lucide:settings" || groups[0].IconReview || groups[1].Icon != "lucide:file-text" || !groups[1].IconReview {
		t.Errorf("groups: %+v", groups)
	}
	if n, _ := s.MigrateIcons(); n != 0 {
		t.Errorf("again: %d changed", n)
	}

	// Picking an icon again clears the review, and puts it first among
	// the recent icons.
	u, _ := s.Get("bbbbbbbbbbbb")
	u.Icon = "lucide:sparkles#e5484d"
	if info, err := s.Save(u); err != nil || info.IconReview {
		t.Fatalf("save: %+v, %v", info, err)
	}
	if _, err := s.Save(proto.Note{NoteInfo: proto.NoteInfo{Title: "x", Icon: "🦄"}}); err == nil || !strings.Contains(err.Error(), "not emoji") {
		t.Errorf("an emoji icon saved: %v", err)
	}
	prefs, err := s.SaveIconPrefs(proto.NoteIconPrefs{Favorites: []string{"lucide:rocket", "nope", "lucide:rocket"}, Libraries: []string{"tabler", "fa"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(prefs.Recent, " ") != "lucide:sparkles#e5484d" || strings.Join(prefs.Favorites, " ") != "lucide:rocket" || strings.Join(prefs.Libraries, " ") != "tabler" {
		t.Errorf("prefs: %+v", prefs)
	}
}
