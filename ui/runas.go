package ui

import (
	"errors"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/hostconn"
)

// defaultRunAs is the user offered to root logins.
const defaultRunAs = "enowx"

// runAsPrompt asks what to do when SSH signs in as root; the connecting
// goroutine waits on answer.
type runAsPrompt struct {
	host   hosts.Host
	users  []hostconn.ServerUser // the server's regular users
	choice string                // one of users, or "" for a new user
	user   string                // the new user's name
	err    string
	answer chan runAsAnswer
}

type runAsAnswer struct {
	host hosts.Host
	err  error
}

var errConnectCancelled = errors.New("connection cancelled")

// askRunAs runs on the connecting goroutine and waits for the user.
// A user enowX already ran as, from this device or another, is chosen
// without asking, so connecting again never makes another user.
func (a *App) askRunAs(gen int, h hosts.Host, users []hostconn.ServerUser) (hosts.Host, error) {
	answer := make(chan runAsAnswer, 1)
	a.win.Update(func() {
		if gen != a.gen {
			answer <- runAsAnswer{err: errConnectCancelled}
			return
		}
		a.runAs = &runAsPrompt{host: h, users: users, user: defaultRunAs, answer: answer}
		if slices.ContainsFunc(users, func(u hostconn.ServerUser) bool { return u.Name == defaultRunAs }) {
			a.runAs.choice = defaultRunAs // made by enowX before, its daemon since removed
		}
		if u, ok := knownServerUser(users); ok {
			a.answerRunAs(u, false)
			a.status = "Running as " + u + ", which enowX set up on " + h.Name + " before."
		}
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

// knownServerUser picks the user enowX ran as before: enowx if it did,
// else the first one.
func knownServerUser(users []hostconn.ServerUser) (string, bool) {
	var known []string
	for _, u := range users {
		if u.HasDaemon {
			known = append(known, u.Name)
		}
	}
	if len(known) == 0 {
		return "", false
	}
	if slices.Contains(known, defaultRunAs) {
		return defaultRunAs, true
	}
	return known[0], true
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
			"enowX can run its daemon and every session as a regular user instead. SSH still signs in as root.").
			FontSize(13).TextColor(colTextSoft)
		submit := false
		if len(p.users) > 0 {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				for _, u := range p.users {
					userOption(c.Key(u.Name), &p.choice, u.Name, u.Name, u.Home)
				}
				userOption(c.Key("+new"), &p.choice, "", "New user", "A user of its own, without sudo")
			})
		}
		if p.choice == "" {
			submit = ui.TextInput(c, &p.user).Font(monoFont).Label("New user").Placeholder(defaultRunAs).Submitted()
		} else {
			ui.Text(c, "Agents get "+p.choice+"'s permissions: if it can use sudo, so can they.").FontSize(12).TextColor(colMuted)
		}
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
			label, user := "Create user and connect", p.user
			if p.choice != "" {
				label, user = "Connect as "+p.choice, p.choice
			}
			if primaryButton(c, iconCheck, label, "").Clicked() || submit {
				a.answerRunAs(user, false)
			}
		})
	})
	if !open {
		a.cancelRunAs()
	}
}

// userOption is a choice of user, marked like the mode cards.
func userOption(c *ui.Context, choice *string, value, title, detail string) {
	selected := *choice == value
	b := ui.ButtonBase(c).FillWidth().Gap(10).Padding(9, 12).Radius(9).Border(1, colBorderHi).Cursor(ui.CursorPointer).Label(title)
	if selected {
		b.Background(colSelected).BorderColor(colAccent)
	} else if b.Hovered() {
		b.Background(colHover)
	}
	b.Children(func() {
		ring := ui.Box(c).Size(14, 14).Radius(7).Border(1.5, colBorderHi).Center().Shrink(0)
		if selected {
			ring.BorderColor(colAccent).Children(func() { ui.Box(c).Size(6, 6).Radius(3).Background(colAccent) })
		}
		ui.Text(c, title).Bold().SingleLine()
		ui.Text(c, detail).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine().Grow(1)
	})
	if b.Clicked() {
		*choice = value
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
