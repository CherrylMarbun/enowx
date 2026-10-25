package notesio

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// sampleStore is notes with groups, a page under another, an icon, an
// image and a link between them.
func sampleStore(t *testing.T) *notes.Store {
	t.Helper()
	s := notes.Open(t.TempDir())
	importInto(t, s, writeTree(t, obsidianVault(t)), Options{}, ApplyOptions{})
	if err := s.SaveGroup("Projects", proto.NoteGroup{Name: "Projects", Icon: "lucide:rocket", Color: "#ff8800"}); err != nil {
		t.Fatal(err)
	}
	return s
}

func exportTo(t *testing.T, s *notes.Store, sel Selection, format string) (string, []byte) {
	t.Helper()
	var b bytes.Buffer
	name, _, err := Export(s, sel, format, &b)
	if err != nil {
		t.Fatal(err)
	}
	return name, b.Bytes()
}

// same says whether two stores hold the same notes: titles, places,
// groups, icons, bodies, files.
func same(t *testing.T, a, b *notes.Store) {
	t.Helper()
	ga, gb := byTitle(t, a), byTitle(t, b)
	if len(ga) != len(gb) {
		t.Fatalf("%d notes, want %d", len(gb), len(ga))
	}
	for title, x := range ga {
		y, ok := gb[title]
		if !ok {
			t.Fatalf("no %q after the round trip", title)
		}
		parent := func(s map[string]proto.Note, id string) string {
			for _, n := range s {
				if n.ID == id {
					return n.Title
				}
			}
			return ""
		}
		if parent(ga, x.Parent) != parent(gb, y.Parent) || x.Group != y.Group || x.Icon != y.Icon || !x.Created.Equal(y.Created) {
			t.Errorf("%q: %+v, want %+v", title, y.NoteInfo, x.NoteInfo)
		}
		if strings.ReplaceAll(x.Body, x.ID, "ID") != strings.ReplaceAll(y.Body, y.ID, "ID") {
			t.Errorf("%q's body:\n%s\nwant\n%s", title, y.Body, x.Body)
		}
		for _, ref := range assetRefs(y) {
			if _, err := b.Asset(ref); err != nil {
				t.Errorf("%q's file %s: %v", title, ref, err)
			}
		}
	}
	gs, _ := b.Groups()
	if len(gs) == 0 || gs[0].Name != "Projects" && gs[1].Name != "Projects" {
		t.Errorf("groups %+v", gs)
	}
}

// An export, as a .zip or a JSON backup, imported on another host makes
// the same notes again.
func TestExportRoundTrip(t *testing.T) {
	for _, format := range []string{proto.ExportZip, proto.ExportJSON} {
		t.Run(format, func(t *testing.T) {
			src := sampleStore(t)
			name, data := exportTo(t, src, Selection{All: true}, format)
			p := filepath.Join(t.TempDir(), name)
			os.WriteFile(p, data, 0o644)
			dst := notes.Open(t.TempDir())
			b, _ := importInto(t, dst, p, Options{}, ApplyOptions{})
			if b.From != proto.ImportBackup {
				t.Fatalf("read as %q", b.From)
			}
			same(t, src, dst)
			for _, g := range must(dst.Groups()) {
				if g.Name == "Projects" && (g.Icon != "lucide:rocket" || g.Color != "#ff8800") {
					t.Errorf("group %+v", g)
				}
			}
			// And again: nothing twice.
			if _, r := importInto(t, dst, p, Options{}, ApplyOptions{}); r.Imported != 0 {
				t.Errorf("imported %d again", r.Imported)
			}
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// The .zip is folders of Markdown, as a vault opens it: a group's notes
// in its folder, a note's under it in its own, files beside them, each
// note's front matter saying what it is.
func TestExportZipLayout(t *testing.T) {
	s := sampleStore(t)
	_, data := exportTo(t, s, Selection{All: true}, proto.ExportZip)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		r, _ := f.Open()
		var b bytes.Buffer
		b.ReadFrom(r)
		files[f.Name] = b.String()
	}
	home := byTitle(t, s)["Home"]
	for _, want := range []string{"Home.md", "Projects/Alpha.md", "Projects/Alpha/Notes.md", "Daily/2024-01-01.md", "assets/" + home.ID + "/cat.png", backupManifest} {
		if _, ok := files[want]; !ok {
			t.Errorf("no %s in %v", want, keys(files))
		}
	}
	contains(t, "Home.md", files["Home.md"], "---\ntitle: Home\n", "enowx: "+home.ID, "](assets/"+home.ID+"/cat.png)", "[[Alpha|the Alpha project]]")
	contains(t, "Notes.md", files["Projects/Alpha/Notes.md"], "title: Notes")
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// One note as Markdown; a group's as a .zip of its notes alone.
func TestExportPicks(t *testing.T) {
	s := sampleStore(t)
	home := byTitle(t, s)["Home"]
	name, data := exportTo(t, s, Selection{IDs: []string{home.ID}}, proto.ExportMarkdown)
	if name != "Home.md" || !strings.HasPrefix(string(data), "---\ntitle: Home") {
		t.Errorf("%s:\n%s", name, data)
	}
	var b bytes.Buffer
	_, n, err := Export(s, Selection{Group: "Projects"}, proto.ExportZip, &b)
	if err != nil || n != 2 {
		t.Errorf("group: %d notes, %v", n, err)
	}
	if _, _, err := Export(s, Selection{IDs: []string{"000000000000"}}, proto.ExportZip, &b); err == nil {
		t.Error("exported a note that isn't")
	}
}

// HTML: one page, contents, the image in it, links between notes kept.
func TestExportHTML(t *testing.T) {
	s := sampleStore(t)
	_, data := exportTo(t, s, Selection{All: true}, proto.ExportHTML)
	alpha := byTitle(t, s)["Alpha"]
	contains(t, "html", string(data), "<nav><h2>Contents</h2>", "<h1>Home</h1>", `<img alt="" src="data:image/png;base64,`,
		`<a href="#n-`+alpha.ID+`">the Alpha project</a>`, `<input type="checkbox" disabled checked> draft`, "<pre><code>// [[not a link]]")
}

// PDF: a file readers open, a page per note, the image in it.
func TestExportPDF(t *testing.T) {
	s := sampleStore(t)
	_, data := exportTo(t, s, Selection{All: true}, proto.ExportPDF)
	text := string(data)
	if !strings.HasPrefix(text, "%PDF-1.4") || !strings.HasSuffix(text, "%%EOF\n") {
		t.Fatal("not a PDF")
	}
	if n := strings.Count(text, "/Type /Page "); n != 4 {
		t.Errorf("%d pages, want 4", n)
	}
	if !strings.Contains(text, "/Subtype /Image") {
		t.Error("no image")
	}
	// Every object the cross-reference names starts where it says.
	xref := text[strings.LastIndex(text, "xref\n"):]
	for i, line := range strings.Split(xref, "\n")[3:] {
		if !strings.HasSuffix(line, " n ") {
			break
		}
		var off int
		if _, err := fmt.Sscan(line, &off); err != nil || !strings.HasPrefix(text[off:], strconv.Itoa(i+1)+" 0 obj") {
			t.Fatalf("object %d isn't at %d", i+1, off)
		}
	}
}

// Notion's API: pages and their blocks, a page under another, a to-do, a
// table and an uploaded image fetched; the token sent to Notion alone.
func TestImportNotionAPI(t *testing.T) {
	img := pngBytes(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/files/chart.png" {
			w.Write(img)
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body any
		switch {
		case r.URL.Path == "/v1/search":
			var q struct {
				Filter struct{ Value string }
			}
			json.NewDecoder(r.Body).Decode(&q)
			if q.Filter.Value == "database" {
				body = map[string]any{"results": []any{}}
				break
			}
			body = map[string]any{"results": []any{
				map[string]any{"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "object": "page", "parent": map[string]any{"type": "workspace"},
					"icon":       map[string]any{"type": "emoji", "emoji": "🚀"},
					"properties": map[string]any{"title": map[string]any{"type": "title", "title": []any{map[string]any{"plain_text": "Launch"}}}}},
				map[string]any{"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "object": "page", "parent": map[string]any{"type": "page_id", "page_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
					"properties": map[string]any{"Name": map[string]any{"type": "title", "title": []any{map[string]any{"plain_text": "Checklist"}}}}},
			}}
		case strings.Contains(r.URL.Path, "aaaaaaaa"):
			rich := func(s string, bold bool) any {
				return map[string]any{"plain_text": s, "annotations": map[string]any{"bold": bold}}
			}
			body = map[string]any{"results": []any{
				map[string]any{"type": "heading_1", "heading_1": map[string]any{"rich_text": []any{rich("Goal", false)}}},
				map[string]any{"type": "paragraph", "paragraph": map[string]any{"rich_text": []any{rich("Ship ", false), rich("v1", true)}}},
				map[string]any{"type": "to_do", "to_do": map[string]any{"checked": true, "rich_text": []any{rich("Plan", false)}}},
				map[string]any{"type": "image", "image": map[string]any{"type": "file", "file": map[string]any{"url": srv.URL + "/files/chart.png"}}},
				map[string]any{"type": "child_page", "id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "child_page": map[string]any{"title": "Checklist"}},
			}}
		default:
			body = map[string]any{"results": []any{}}
		}
		json.NewEncoder(w).Encode(body)
	}))
	defer srv.Close()
	s := notes.Open(t.TempDir())
	b, err := Read(context.Background(), "", Options{From: proto.ImportNotionAPI, Token: "secret-token", NotionAPI: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Apply(context.Background(), s, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	got := byTitle(t, s)
	launch := got["Launch"]
	if launch.Icon != "lucide:rocket" || got["Checklist"].Parent != launch.ID {
		t.Fatalf("notes %+v", got)
	}
	contains(t, "launch", launch.Body, "# Goal", "Ship **v1**", "- [x] Plan", "](assets/"+launch.ID+"/chart.png)", "[[Checklist]]")
	if _, err := Read(context.Background(), "", Options{From: proto.ImportNotionAPI, Token: "wrong", NotionAPI: srv.URL}); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("a wrong token: %v", err)
	}
}

// A note's SVG icon, its icons in the text and its projects go out with
// its Markdown, readable elsewhere.
func TestExportMarkdownCarriesIcons(t *testing.T) {
	n := proto.Note{NoteInfo: proto.NoteInfo{ID: "abcdefabcdef", Title: "Plan", Icon: "lucide:rocket",
		Projects: []proto.NoteProject{{Name: "api", HostName: "vps"}}}, Body: "Done :icon[lucide:check] here\n"}
	md := markdownFile(n, func(ref string) string { return ref })
	for _, want := range []string{`icon: "lucide:rocket"`, `icon-svg: "data:image/svg+xml;base64,`, "projects: [api · vps]", `<img alt="check" src="data:image/svg+xml;base64,`} {
		if !strings.Contains(md, want) {
			t.Errorf("no %q in:\n%s", want, md)
		}
	}
}
