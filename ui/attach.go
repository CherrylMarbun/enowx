package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/ui/chatview"
	"github.com/enowx/enowx/ui/textfmt"
)

// Images and files given to an agent (pasted into the chat or the
// terminal, a Browser screenshot) reach it as paths of the session's host:
// an image is saved to a file here, and a server's session gets the files
// uploaded first. Agents read images from paths typed into their prompt.

// saveImage saves png as a file of this device, named after what it is.
func saveImage(png []byte, kind string) (string, error) {
	dir, err := transport.Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "screenshots", kind+"-"+time.Now().Format("20060102-150405.000")+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, png, 0o600)
}

// clipboardFiles is what a paste of d gives an agent: the image it holds,
// saved to a file, or the files copied. None when it holds text, which
// pastes as text.
func clipboardFiles(d transfer.Data) ([]string, error) {
	if files, err := d.Files(); err == nil && len(files) > 0 {
		return files, nil
	}
	if _, ok := d.Preferred(transfer.PNG); !ok {
		return nil, nil
	}
	if _, ok := d.Preferred(transfer.Text); ok {
		return nil, nil // text with a picture of it (a spreadsheet's cells)
	}
	png, err := d.Read(transfer.PNG)
	if err != nil || len(png) == 0 {
		return nil, errors.New("the clipboard's image can't be read")
	}
	path, err := saveImage(png, "pasted")
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}

// giveFiles makes local files paths session s's agent can open, uploading
// them to a server's session, off the main thread; done gets the paths,
// escaped for a prompt and spaced, on the main thread.
func (a *App) giveFiles(s proto.SessionInfo, local []string, done func(paths string)) {
	cl := a.client
	if cl == nil || len(local) == 0 {
		return
	}
	go func() {
		paths := make([]string, len(local))
		for i, p := range local {
			paths[i] = p
			if cl.Remote() {
				remote, err := cl.Upload(s.ID, p)
				if err != nil {
					a.onMain(func() { a.status = "Couldn't send " + filepath.Base(p) + ": " + client.Explain(err) })
					return
				}
				paths[i] = remote
			}
		}
		for i, p := range paths {
			paths[i] = escapePath(p)
		}
		a.onMain(func() {
			done(strings.Join(paths, " ") + " ")
			a.notify(textfmt.Plural(len(paths), "A file", "Files") + " attached to " + a.sessionName(s) + "'s prompt")
		})
	}()
}

// pasteFiles takes a paste of the clipboard's image or files for session
// s, giving their paths to insert; it reports whether it took the paste,
// which leaves text to paste as text.
func (a *App) pasteFiles(s proto.SessionInfo, d transfer.Data, insert func(paths string)) bool {
	files, err := clipboardFiles(d)
	if err != nil {
		a.status = err.Error()
		return true
	}
	if len(files) == 0 {
		return false
	}
	a.giveFiles(s, files, insert)
	return true
}

// chatPaste takes a paste into session s's chat composer: text goes in
// as text; an image or files are attached to the message, shown as small
// previews while they upload.
func (a *App) chatPaste(s proto.SessionInfo, ch *chatview.Chat, text string) bool {
	if text != "" {
		return false
	}
	d, err := mygo.Clipboard.Read(transfer.PNG, transfer.FileList, transfer.URIList, transfer.Text)
	if err != nil {
		return false
	}
	files, err := clipboardFiles(d)
	if err != nil {
		a.status = err.Error()
		return true
	}
	for _, f := range files {
		done := ch.Attach(filepath.Base(f), imagePreview(f))
		a.uploadFile(s, f, func(path string, err error) {
			if err != nil {
				a.status = "Couldn't attach " + filepath.Base(f) + ": " + client.Explain(err)
				done("", err)
				return
			}
			done(escapePath(path), nil)
		})
	}
	return len(files) > 0
}

// maxPreview bounds the image files read for a preview.
const maxPreview = 8 << 20

// imagePreview is the bytes of the image file at path, for a preview; nil
// for another file, or one too large.
func imagePreview(path string) []byte {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
	default:
		return nil
	}
	if info, err := os.Stat(path); err != nil || info.Size() > maxPreview {
		return nil
	}
	data, _ := os.ReadFile(path)
	return data
}

// uploadFile makes local file a path session s's agent can open, uploading
// it to a server's session, off the main thread; done runs on it.
func (a *App) uploadFile(s proto.SessionInfo, local string, done func(path string, err error)) {
	cl := a.client
	if cl == nil {
		done("", errors.New("not connected"))
		return
	}
	if !cl.Remote() {
		done(local, nil)
		return
	}
	go func() {
		remote, err := cl.Upload(s.ID, local)
		a.onMain(func() { done(remote, err) })
	}()
}
