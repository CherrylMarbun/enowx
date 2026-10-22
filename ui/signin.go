package ui

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/transport"
)

// signInPrompt asks again how to sign in after a server refused: another
// user, or a password, as ssh would prompt for.
type signInPrompt struct {
	host     hosts.Host
	refusal  *transport.AuthError
	user     string
	password string
	remember bool
	err      string
}

// promptSignIn opens the sign-in dialog when err is a refused sign-in.
func (a *App) promptSignIn(h hosts.Host, err error) {
	var refusal *transport.AuthError
	if h.ID == hosts.LocalID || !errors.As(err, &refusal) {
		return
	}
	a.signIn = &signInPrompt{host: h, refusal: refusal, user: refusal.User, remember: true}
}

func (a *App) submitSignIn() {
	p := a.signIn
	user := strings.TrimSpace(p.user)
	if user == "" || strings.ContainsAny(user, "@ ") {
		p.err = "Type the user to sign in as."
		return
	}
	if p.refusal.AcceptsPassword() && p.password == "" {
		p.err = "Type the password."
		return
	}
	t, err := transport.ParseSSHTarget(p.host.Address)
	if err != nil {
		p.err = err.Error()
		return
	}
	t.User = user
	h := p.host
	h.Address = t.String()
	password := p.password
	if password != "" && p.remember {
		h.PasswordAuth = true
	}
	if _, stored := a.storedHost(h.ID); stored {
		if err := a.opts.Hosts.Update(h); err != nil {
			p.err = err.Error()
			return
		}
		a.reloadHosts()
	}
	a.signIn = nil
	conn := h
	if password != "" && !p.remember {
		// Kept for this run of the app only.
		if a.passwords == nil {
			a.passwords = map[string]string{}
		}
		a.passwords[h.ID] = password
		conn.PasswordAuth = true
	}
	a.connect(conn)
	if password != "" && p.remember {
		// After connect, which clears the status bar: a warning about the
		// keychain must stay visible.
		a.savePassword(h, password)
	}
}

func (a *App) signInDialog(c *ui.Context) {
	p := a.signIn
	if p == nil {
		return
	}
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(480).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi)
		ui.Text(c, "Sign in to "+p.host.Name).FontSize(16).Bold()
		what := "your SSH keys"
		if p.refusal.TriedPassword {
			what = "your SSH keys or the saved password"
		}
		ui.Text(c, p.refusal.Addr+" didn't accept "+what+" for "+p.refusal.User+".").FontSize(13).TextColor(colTextSoft)
		submit := false
		field(c, "User", func() {
			in := ui.TextInput(c, &p.user).Font(monoFont).Placeholder("root")
			if !p.refusal.AcceptsPassword() {
				in.AutoFocus()
			}
			submit = in.Submitted()
		})
		if p.refusal.AcceptsPassword() {
			field(c, "Password", func() {
				submit = ui.TextInput(c, &p.password).Password().AutoFocus().Placeholder("The user's password").Submitted() || submit
			})
			ui.Row(c).FillWidth().Gap(10).Children(func() {
				ui.Text(c, "Remember the password in the keychain").FontSize(13).TextColor(colTextSoft).Grow(1)
				ui.Switch(c, &p.remember).Label("Remember the password in the keychain")
			})
		} else {
			ui.Text(c, "This server accepts SSH keys only. Add your public key (~/.ssh/id_ed25519.pub) to the user's ~/.ssh/authorized_keys, or allow passwords in the server's /etc/ssh/sshd_config.").
				FontSize(12).TextColor(colMuted)
		}
		if p.err != "" {
			ui.Text(c, p.err).FontSize(12).TextColor(colError)
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if ghostButton(c, "Cancel").Clicked() {
				a.signIn = nil
			}
			if primaryButton(c, iconCheck, "Sign in", "").Clicked() || submit {
				a.submitSignIn()
			}
		})
	})
	if !open {
		a.signIn = nil
	}
}
