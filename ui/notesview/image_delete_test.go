package notesview

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDeleteImagePreservesCompanionAndOtherBlocks(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"inline", "Before\n![Image](assets/delete/shot.png) Keep this text\nAfter\n"},
		{"columns", "Before\n:::columns\n::col\n![Image](assets/delete/shot.png)\n::col\n## Keep this text\n- [ ] Keep this task\n:::\nAfter\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEditor(tc.body)
			tt := imageEditingTester(t, e)
			tt.Frame()
			if err := tt.Click("Delete Image 1"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			body := e.markdown()
			if strings.Contains(body, "![Image]") || strings.Contains(body, ":::columns") || !strings.Contains(body, "Before") || !strings.Contains(body, "After") || !strings.Contains(body, "Keep this text") {
				t.Fatalf("deleting an image lost its text or left its layout: %q", body)
			}
			if tc.name == "columns" && !strings.Contains(body, "- [ ] Keep this task") {
				t.Fatal("task was lost")
			}
			reopened := newEditor(body)
			if len(reopened.noteImages()) != 0 {
				t.Fatal("deleted image returned after reopening")
			}
		})
	}
}

func TestDeleteImageFromContextMenu(t *testing.T) {
	e := newEditor("![Image](assets/delete-menu/shot.png)\nAfter\n")
	tt := imageEditingTester(t, e)
	waitForTestImage(t, tt)
	if err := tt.RightClick("Block 1"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Delete image"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := e.markdown(); got != "After\n" {
		t.Fatalf("context menu delete: %q", got)
	}
}

func TestBackspaceDeletesEmptyImageAndRestoresNormalEditor(t *testing.T) {
	e := newEditor("![Image](assets/delete-key/shot.png)\n")
	tt := imageEditingTester(t, e)
	e.openAt(0, -1)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if len(e.blocks) != 1 || e.blocks[0].text != "" {
		t.Fatalf("image remains after Backspace: %q", e.markdown())
	}
	tt.Type("/todo")
	tt.Frame()
	if !tt.HasText("To-do list") {
		t.Fatalf("normal editing did not return: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Type("Continue writing")
	tt.Frame()
	if got := e.markdown(); got != "- [ ] Continue writing\n" {
		t.Fatalf("normal blocks after image removal: %q", got)
	}
	shot(t, tt, "after-deleting-image.png")
}

func TestDeleteImageWithKeysFromAdjacentBlocks(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		at         int
		key        ui.Key
	}{
		{"Delete before image", "Before\n![Image](assets/forward-delete/shot.png)\n", 0, ui.KeyDelete},
		{"Backspace after image", "![Image](assets/backward-delete/shot.png)\nAfter\n", 1, ui.KeyBackspace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEditor(tc.body)
			tt := imageEditingTester(t, e)
			caret := 0
			if tc.key == ui.KeyDelete {
				caret = -1
			}
			e.openAt(tc.at, caret)
			tt.Frame()
			tt.Key(0, tc.key)
			tt.Frame()
			if len(e.noteImages()) != 0 || len(e.blocks) != 1 {
				t.Fatalf("keyboard removal: %q", e.markdown())
			}
		})
	}
}

func TestDeleteOnlyChosenImage(t *testing.T) {
	e := newEditor("![First](assets/chosen/first.png) one ![Second](assets/chosen/second.png) two\n")
	tt := imageEditingTester(t, e)
	tt.Frame()
	if err := tt.Click("Delete Second"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := e.noteImages(); len(got) != 1 || got[0].name != "First" {
		t.Fatalf("deleted another image: %+v", got)
	}
	if !strings.Contains(e.markdown(), "one") || !strings.Contains(e.markdown(), "two") {
		t.Fatal("companion text was lost")
	}
}
