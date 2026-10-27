package ui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

func (a *App) pickNoteImage(cl *client.Client, id string, done func(string, error)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: "Add an image to this note",
			Filters: []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"}}}})
		var ref string
		if err == nil && len(paths) > 0 {
			var data []byte
			data, err = readNoteImage(paths[0])
			if err == nil {
				ref, err = cl.AddNoteImage(id, filepath.Base(paths[0]), data)
			}
		}
		a.onMain(func() { done(ref, err) })
	}()
}

func (a *App) pasteNoteImage(cl *client.Client, id string, done func(string, error)) bool {
	data := mygo.Clipboard.ReadImage()
	if len(data) == 0 {
		return false
	}
	go func() {
		ref, err := cl.AddNoteImage(id, "pasted-image.png", data)
		a.onMain(func() { done(ref, err) })
	}()
	return true
}

func readNoteImage(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, proto.MaxNoteImage+1))
	if err == nil && len(data) > proto.MaxNoteImage {
		err = fmt.Errorf("choose an image up to 8 MB")
	}
	return data, err
}
