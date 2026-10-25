package ui

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/chatview"
)

// Pictures given to a canvas's agent ("make it like this") are kept with
// the canvas, as its assets, so its agent reads them on any host and they
// sync with it.

// canvasPaste attaches the pictures pasted into the chat, kept with canvas
// id (its assets), so its agent reads them on any host.
func (a *App) canvasPaste(id string, ch *chatview.Chat, text string) bool {
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
		a.attachToCanvas(id, ch, f)
	}
	return len(files) > 0
}

// attachToCanvas uploads local file f to canvas id and attaches its path
// on the host to ch's next message.
func (a *App) attachToCanvas(id string, ch *chatview.Chat, f string) {
	cl := a.client
	done := ch.Attach(filepath.Base(f), imagePreview(f))
	go func() {
		data, err := os.ReadFile(f)
		var asset proto.CanvasAsset
		if err == nil {
			asset, err = cl.AddCanvasAssetAt(id, "chat-"+filepath.Base(f), data)
		}
		a.onMain(func() {
			if err != nil {
				a.status = "Couldn't attach " + filepath.Base(f) + ": " + client.Explain(err)
				done("", err)
				return
			}
			done(escapePath(asset.Path), nil)
		})
	}()
}

// canvasAttachFile asks for pictures to attach to the next message.
func (a *App) canvasAttachFile(id string, s proto.SessionInfo, ok bool) {
	win := a.win
	agentID := a.canvasAgentOf(a.canvasChatOf(id).log)
	key := "canvas-new:" + id + ":" + agentID
	if ok {
		key = s.ID
	}
	ch := a.chatOf(key)
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Title: "Attach a picture", Multiple: true})
		if err != nil || len(paths) == 0 {
			return
		}
		a.onMain(func() {
			for _, p := range paths {
				a.attachToCanvas(id, ch, p)
			}
		})
	}()
}
