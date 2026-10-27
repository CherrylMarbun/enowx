package notesview

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/enowx/enowx/proto"
)

func TestPasteImageKeepsEditsDuringUpload(t *testing.T) {
	p := &page{id: "abcdef123456", ed: newEditor("Before\n")}
	v := &View{page: p}
	var finish func(string, error)
	o := Options{PasteImage: func(id string, done func(string, error)) bool {
		finish = done
		return true
	}}
	p.ed.pasteImage = func() bool { return v.pasteImage(o, p) }
	tt := newTester(p.ed)
	p.ed.openAt(0, -1)
	tt.Frame()
	tt.SetClipboard("")
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if finish == nil || !p.uploading {
		t.Fatal("paste did not start an image upload")
	}
	p.ed.blocks[0].text = "Edited during upload"
	finish("assets/abcdef123456/image.png", nil)
	if !strings.Contains(p.ed.markdown(), "Edited during upload\n\n![Image](assets/abcdef123456/image.png)") || p.uploading {
		t.Fatalf("after upload: %q", p.ed.markdown())
	}
}

func TestImageCompletesAfterChangingPages(t *testing.T) {
	p := &page{id: "abcdef123456", ed: newEditor("Original\n")}
	v := &View{page: &page{}}
	var saved proto.Note
	o := Options{Save: func(n proto.Note, done func(proto.NoteInfo, error)) {
		saved = n
		done(proto.NoteInfo{ID: p.id}, nil)
	}}
	v.imageDone(o, p)("assets/abcdef123456/image.png", nil)
	if !strings.Contains(saved.Body, "![Image]") {
		t.Fatal("image was lost after switching pages")
	}
}

func TestImageUploadErrorAndCancelKeepBody(t *testing.T) {
	p := &page{ed: newEditor("Keep this\n"), uploading: true}
	v := &View{page: p}
	finish := v.imageDone(Options{}, p)
	finish("", fmt.Errorf("upload failed"))
	if p.uploading || v.err == "" || p.ed.markdown() != "Keep this\n" {
		t.Fatal("upload failure changed the body or left the upload active")
	}
	p.uploading = true
	finish("", nil)
	if p.uploading || p.ed.markdown() != "Keep this\n" {
		t.Fatal("canceling changed the body or left the upload active")
	}
}
