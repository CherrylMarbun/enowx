package notesio

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// Sources as the apps write them, made in the test's folder.

// pngBytes is a small real image.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for x := range 4 {
		img.Set(x, 1, color.RGBA{200, 30, 30, 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// writeTree writes files (by slash path) under a new folder.
func writeTree(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// writeZip writes files into a .zip, as the apps hand their exports out.
func zipOf(t *testing.T, name string, files map[string][]byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for n, data := range files {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const (
	nPlan    = "1a2b3c4d5e6f708192a3b4c5d6e7f801"
	nMeeting = "2b3c4d5e6f708192a3b4c5d6e7f80112"
	nRoad    = "3c4d5e6f708192a3b4c5d6e7f8011223"
	nLogin   = "4d5e6f708192a3b4c5d6e7f801122334"
	nSearch  = "5e6f708192a3b4c5d6e7f80112233445"
)

// notionExport is a Notion export, Markdown & CSV: a page with a page and
// a database under it, the database's rows pages of their own, an image.
func notionExport(t *testing.T) map[string][]byte {
	plan := "Project Plan " + nPlan
	road := plan + "/Roadmap " + nRoad
	return map[string][]byte{
		plan + ".md": []byte("# Project Plan\n\n<aside>\n💡 Keep it small.\n</aside>\n\n## Tasks\n\n- [x] Write the spec\n- [ ] Ship it\n\n" +
			"See [Meeting notes](Project%20Plan%20" + nPlan + "/Meeting%20notes%20" + nMeeting + ".md) and the [Roadmap](Project%20Plan%20" + nPlan + "/Roadmap%20" + nRoad + ".csv).\n\n" +
			"![Diagram](Project%20Plan%20" + nPlan + "/diagram.png)\n\n| Name | Owner |\n| --- | --- |\n| API | Ada |\n"),
		plan + "/Meeting notes " + nMeeting + ".md": []byte("# Meeting notes\n\nBack to the [plan](../Project%20Plan%20" + nPlan + ".md).\n"),
		plan + "/diagram.png":                       pngBytes(t),
		road + ".csv":                               []byte("\ufeffName,Status,Owner\nLogin,Done,Ada\nSearch,Planned,Grace\n"),
		road + "_all.csv":                           []byte("\ufeffName,Status,Owner\nLogin,Done,Ada\nSearch,Planned,Grace\n"),
		road + "/Login " + nLogin + ".md":           []byte("# Login\n\nStatus: Done\nOwner: Ada\n\nUse passkeys first.\n"),
		road + "/Search " + nSearch + ".md":         []byte("# Search\n\nStatus: Planned\nOwner: Grace\n"),
	}
}

// obsidianVault is a vault: a home note linking notes in folders, a
// folder's note beside it, an attachment shown by name, daily notes.
func obsidianVault(t *testing.T) map[string][]byte {
	return map[string][]byte{
		"Vault/.obsidian/app.json": []byte(`{"attachmentFolderPath":"attachments"}`),
		"Vault/Home.md": []byte("---\ntags:\n  - start\n---\n# Home\n\n## Intro\n\nWelcome. Work is in [[Alpha|the Alpha project]] and [[Notes]].\n\n" +
			"![[cat.png]]\n\n```go\n// [[not a link]]\n```\n"),
		"Vault/Projects/Alpha.md":       []byte("---\ntitle: Alpha\ncreated: 2024-03-01\n---\nThe plan. Back [[Home#Intro]].\n"),
		"Vault/Projects/Alpha/Notes.md": []byte("- [ ] call Ada\n- [x] draft\n"),
		"Vault/Daily/2024-01-01.md":     []byte("Met [[Home]] people. See [the plan](../Projects/Alpha.md).\n"),
		"Vault/attachments/cat.png":     pngBytes(t),
	}
}

const enex = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE en-export SYSTEM "http://xml.evernote.com/pub/evernote-export3.dtd">
<en-export export-date="20240101T120000Z" application="Evernote">
<note><title>Groceries</title><created>20231201T080000Z</created><updated>20231202T090000Z</updated><tag>home</tag>
<content><![CDATA[<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<!DOCTYPE en-note SYSTEM "http://xml.evernote.com/pub/enml2.dtd">
<en-note><div><b>Buy</b> these:</div><div><en-todo checked="true"/>Milk</div><div><en-todo/>Bread</div>
<div><en-media hash="{{HASH}}" type="image/png"/></div><script>alert(1)</script></en-note>]]></content>
<resource><data encoding="base64">{{DATA}}</data><mime>image/png</mime><resource-attributes><file-name>receipt.png</file-name></resource-attributes></resource>
</note>
</en-export>`

const keepJSON = `{"color":"DEFAULT","isTrashed":false,"isPinned":false,"isArchived":false,
"title":"Packing","listContent":[{"text":"Passport","isChecked":true},{"text":"Charger","isChecked":false}],
"labels":[{"name":"Travel"}],"createdTimestampUsec":1700000000000000,"userEditedTimestampUsec":1700000100000000}`

const appleHTML = `<html><head><title>Recipe</title><style>p{}</style></head><body>
<h1>Recipe</h1><p>Mix <i>flour</i> and <a href="javascript:alert(1)">water</a>, then <a href="https://example.com/bake">bake</a>.</p>
<ol><li>Mix</li><li>Bake<ul><li>at 200°</li></ul></li></ol>
<blockquote>Best warm.</blockquote><iframe src="https://example.com/video"></iframe>
<table><tr><th>Item</th><th>Grams</th></tr><tr><td>Flour</td><td>500</td></tr></table></body></html>`
