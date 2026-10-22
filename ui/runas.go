package ui

import (
	"errors"

	"github.com/egoist/mygo/ui"
	"github.com/enowx/enowx/core/hosts"
)

// defaultRunAs is the user offered to root logins.
const defaultRunAs = "enowx"

// runAsPrompt asks what to do when SSH signs in as root; the connecting
// goroutine waits on answer.
type runAsPrompt struct {
	host   hosts.Host
	user   string
	err    string
	answer chan runAsAnswer
}

type runAsAnswer struct {
	host hosts.Host
	err  error
}

var errConnectCancelled = errors.New("connection cancelled")

// askRunAs runs on the connecting goroutine and waits for the user.
func (a *App) askRunAs(gen int, h hosts.Host) (hosts.Host, error) {
	answer := make(chan runAsAnswer, 1)
	a.win.Update(func() {
		if gen != a.gen {
			answer <- runAsAnswer{err: errConnectCancelled}
			return
		}
		a.runAs = &runAsPrompt{host: h, user: defaultRunAs, answer: answer}
	})
	r := <-answer
	return r.host, r.err
}

// answerRunAs saves the choice for next time and lets the connection go on.
func (a *App) answerRunAs(user string, keepRoot bool) {
	p := a.runAs
	if p == nil {
		return
	}
	h := p.host
	h.RunAs, h.KeepRoot = user, keepRoot
	if !keepRoot && !hosts.ValidUserName(user) {
		p.err = "Use lowercase letters, digits, - and _, starting with a letter."
		return
	}
	if a.opts.Hosts != nil {
		if _, ok := a.opts.Hosts.Get(h.ID); ok {
			if err := a.opts.Hosts.Update(h); err != nil {
				p.err = err.Error()
				return
			}
			a.reloadHosts()
		}
	}
	a.host = h
	p.answer <- runAsAnswer{host: h}
	a.runAs = nil
}

func (a *App) cancelRunAs() {
	if a.runAs != nil {
		a.runAs.answer <- runAsAnswer{err: errConnectCancelled}
		a.runAs = nil
	}
}

func (a *App) runAsDialog(c *ui.Context) {
	p := a.runAs
	if p == nil {
		return
	}
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(540).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi)
		ui.Text(c, "Run agents as a regular user?").FontSize(16).Bold()
		ui.Text(c, "You signed in to "+p.host.Name+" as root. Claude Code won't skip permission prompts as root, and agents running as root can change anything on the server. "+
			"enowX can create a regular user and run its daemon and every session as that user. SSH still signs in as root.").
			FontSize(13).TextColor(colTextSoft)
		submit := ui.TextInput(c, &p.user).Font(monoFont).Label("User").Placeholder(defaultRunAs).Submitted()
		if p.err != "" {
			ui.Text(c, p.err).FontSize(12).TextColor(colError)
		}
		ui.Text(c, "Afterwards, sign in to each agent once from a Shell session (it runs as that user), and keep projects in that user's home. "+
			"Agents installed only for root, for example with nvm, need installing again.").FontSize(12).TextColor(colMuted)
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if ghostButton(c, "Cancel").Clicked() {
				a.cancelRunAs()
			}
			if ghostButton(c, "Keep root").Clicked() {
				a.answerRunAs("", true)
			}
			if primaryButton(c, iconCheck, "Create user and connect", "").Clicked() || submit {
				a.answerRunAs(p.user, false)
			}
		})
	})
	if !open {
		a.cancelRunAs()
	}
}

// runAsNote says who enowX runs as on a root login, once the user chose.
func runAsNote(h hosts.Host) string {
	switch {
	case h.RunAs != "":
		return "Runs as " + h.RunAs + " when signed in as root"
	case h.KeepRoot:
		return "Runs as root"
	}
	return ""
}
