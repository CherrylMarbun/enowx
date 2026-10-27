package daemon

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func TestNoteImageUpload(t *testing.T) {
	c := connect(t, startServer(t), nil)
	n, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Pictures"}})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	ref, err := c.AddNoteImage(n.ID, "../../a picture.jpg", data.Bytes())
	if err != nil || ref != "assets/"+n.ID+"/a-picture.png" {
		t.Fatalf("upload: %q, %v", ref, err)
	}
	got, err := c.NoteAsset(ref)
	if err != nil || !bytes.Equal(got, data.Bytes()) {
		t.Fatalf("read uploaded image: %v", err)
	}
	// Reusing a name for different content must preserve the earlier image.
	var other bytes.Buffer
	png.Encode(&other, image.NewRGBA(image.Rect(0, 0, 10, 20)))
	ref2, err := c.AddNoteImage(n.ID, "a picture.png", other.Bytes())
	if err != nil || ref2 == ref {
		t.Fatalf("second upload: %q, %v", ref2, err)
	}
	got, _ = c.NoteAsset(ref)
	if !bytes.Equal(got, data.Bytes()) {
		t.Fatal("second upload replaced the first image")
	}
	full, _ := c.Note(n.ID)
	full.Body = "![Image](" + ref + ")\n"
	if _, err := c.SaveNote(full); err != nil {
		t.Fatal(err)
	}
	full, _ = c.Note(n.ID)
	if !strings.Contains(full.Body, ref) {
		t.Fatal("image reference was not saved")
	}
	for _, tc := range []struct {
		name, id string
		data     []byte
	}{
		{"invalid image", n.ID, []byte("not an image")},
		{"oversize", n.ID, make([]byte, proto.MaxNoteImage+1)},
		{"missing note", "abcdef123456", data.Bytes()},
		{"invalid ID", "../outside", data.Bytes()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.AddNoteImage(tc.id, "image.png", tc.data); err == nil {
				t.Fatal("invalid upload succeeded")
			}
		})
	}
}

func TestNoteImageReadByAbsoluteHostPath(t *testing.T) {
	srv := startServer(t)
	c := connect(t, srv, nil)
	n, err := c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Absolute image"}})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	ref, err := c.AddNoteImage(n.ID, "icon.png", data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	sess, err := c.CreateSession(proto.SessionCreateParams{Command: []string{os.Args[0], "-test.run=^$"}, Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(srv.notes.Dir(), filepath.FromSlash(ref))
	for _, p := range []string{abs, filepath.ToSlash(abs)} {
		got, err := c.ReadFile(sess.ID, p, time.Time{})
		if err != nil || got.Image != "png" || !bytes.Equal(got.Data, data.Bytes()) {
			t.Fatalf("absolute note image %q: format=%q, error=%v", p, got.Image, err)
		}
	}
}
