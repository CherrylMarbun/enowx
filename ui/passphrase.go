package ui

import (
	"errors"
	"os"
	"strings"

	"github.com/egoist/mygo/ui"
)

// passphrasePrompt asks for the passphrase of an SSH key; the connecting
// goroutine waits on answer. Passphrases are never saved.
type passphrasePrompt struct {
	path   string
	value  string
	answer chan passphraseAnswer
}

type passphraseAnswer struct {
	value string
	err   error
}

// askPassphrase runs on the connecting goroutine and waits for the user.
func (a *App) askPassphrase(gen int, path string) (string, error) {
	answer := make(chan passphraseAnswer, 1)
	a.win.Update(func() {
		if gen != a.gen {
			answer <- passphraseAnswer{err: errConnectCancelled}
			return
		}
		a.passphrase = &passphrasePrompt{path: path, answer: answer}
	})
	r := <-answer
	return r.value, r.err
}

func (a *App) answerPassphrase(ok bool) {
	p := a.passphrase
	if p == nil {
		return
	}
	if ok {
		p.answer <- passphraseAnswer{value: p.value}
	} else {
		p.answer <- passphraseAnswer{err: errors.New("no passphrase for " + p.path)}
	}
	a.passphrase = nil
}

func (a *App) passphraseDialog(c *ui.Context) {
	p := a.passphrase
	if p == nil {
		return
	}
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(480).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi)
		ui.Text(c, "Unlock your SSH key").FontSize(16).Bold()
		ui.Text(c, "The server accepts this key. Type its passphrase to sign in; enowX doesn't save it.").
			FontSize(13).TextColor(colTextSoft)
		ui.Text(c, shortHome(p.path)).Font(monoFont).FontSize(12).Padding(10).Radius(8).Background(colBackground).FillWidth()
		submit := ui.TextInput(c, &p.value).Password().AutoFocus().Label("Passphrase").Placeholder("Passphrase").Submitted()
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if ghostButton(c, "Cancel").Clicked() {
				a.answerPassphrase(false)
			}
			if primaryButton(c, iconCheck, "Unlock and connect", "").Clicked() || submit {
				a.answerPassphrase(true)
			}
		})
	})
	if !open {
		a.answerPassphrase(false)
	}
}

// shortHome writes paths under the home folder with ~.
func shortHome(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
