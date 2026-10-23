package ui

import (
	"bytes"
	"os"
	"testing"

	"github.com/egoist/mygo/transfer"
)

// A paste of an image gives the agent a file of it; one of text stays
// text, even with a picture of that text (a spreadsheet's cells).
func TestClipboardFiles(t *testing.T) {
	t.Setenv("ENOWX_HOME", t.TempDir())
	png := []byte("\x89PNG\r\n\x1a\nimage")
	files, err := clipboardFiles(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, png))))
	if err != nil || len(files) != 1 {
		t.Fatalf("image: %v, %v", files, err)
	}
	if data, _ := os.ReadFile(files[0]); !bytes.Equal(data, png) {
		t.Fatalf("saved %q", data)
	}
	text := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("A1\tB1")), transfer.Bytes(transfer.PNG, png)))
	if files, err := clipboardFiles(text); err != nil || len(files) != 0 {
		t.Fatalf("text: %v, %v", files, err)
	}
}
