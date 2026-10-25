package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/enowx/enowx/proto"
)

// An image a note shows from its own files (as an import brings them)
// shows on its page, read from its host.
func TestNoteShowsItsImages(t *testing.T) {
	h := newNotesHarness(t)
	n, err := h.cl.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Trip"}, Body: "Before\n"})
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	dir := filepath.Join(h.dir, "notes", "assets", n.ID)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "view.png"), buf.Bytes(), 0o644)
	full, _ := h.cl.Note(n.ID)
	full.Body = "Before\n\n![the view](assets/" + n.ID + "/view.png)\n"
	h.cl.SaveNote(full)
	if _, err := h.cl.NoteAsset("assets/" + n.ID + "/view.png"); err != nil {
		t.Fatalf("asset: %v", err)
	}
	h.settle("the page listed", func() bool { return h.tt.HasText("Trip") })
	h.tt.Click("Trip")
	h.settle("the image shown", func() bool {
		return h.tt.HasText("Before") && !h.tt.HasText("Loading the image…") && !h.tt.HasText("The image didn't load")
	})
	if h.tt.HasText("![the view]") {
		t.Error("the image shows as its Markdown")
	}
}
