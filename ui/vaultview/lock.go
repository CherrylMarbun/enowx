package vaultview

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// create is the page before there is a vault: a master password, twice.
func (v *View) create(c *ui.Context, o Options) {
	v.hero(c, icon.Shield, "One vault for your logins",
		"Keep tokens, keys and passwords for GitHub, your clouds, databases and AI APIs once, encrypted on this device. Sessions on any host get what they need without logging in again: the first time on each host, you allow it.", func() {
			widget.Field(c, "Master password", func() {
				widget.Input(c, &v.password, "At least 8 characters").FillWidth().Password().AutoFocus()
			})
			var submit bool
			widget.Field(c, "Again", func() {
				submit = widget.Input(c, &v.confirm, "The same").FillWidth().Password().Submitted()
			})
			widget.Switch(c, &v.remember, "Remember on this device")
			ui.Text(c, rememberNote).FontSize(11.5).TextColor(theme.Muted)
			v.errorLine(c)
			if widget.PrimaryButton(c, icon.Shield, "Make the vault", "").Clicked() || submit {
				switch {
				case len(v.password) < 8:
					v.err = "The master password needs at least 8 characters."
				case v.password != v.confirm:
					v.err = "The two passwords differ."
				default:
					v.err = errText(o.Actions.Create(v.password, v.remember))
					if v.err == "" {
						v.password, v.confirm = "", ""
					}
				}
			}
			ui.Text(c, "There is no way back in without it: enowX can't recover a forgotten master password.").FontSize(11.5).TextColor(theme.Warning)
		})
}

const rememberNote = "Its key is kept in the system's keychain, so the vault opens without the password; it still locks after a while unused."

// unlock is the page while the vault is locked.
func (v *View) unlock(c *ui.Context, o Options) {
	if !v.remember && o.Remember {
		v.remember = true
	}
	v.hero(c, icon.Lock, "The vault is locked", "Unlock it to see your credentials and lend them to sessions.", func() {
		submit := false
		widget.Field(c, "Master password", func() {
			submit = widget.Input(c, &v.password, "Your master password").FillWidth().Password().AutoFocus().Submitted()
		})
		widget.Switch(c, &v.remember, "Remember on this device")
		v.errorLine(c)
		if widget.PrimaryButton(c, icon.Lock, "Unlock", "").Clicked() || submit {
			v.err = errText(o.Actions.Unlock(v.password, v.remember))
			v.password = "" // wrong: typed again from the start
		}
	})
}

// hero is a centered card: a mark, a title, a line and a form.
func (v *View) hero(c *ui.Context, svg *ui.SVG, title, about string, body func()) {
	ui.Column(c).FillWidth().AlignItems(ui.Center).Padding(32, 0).Children(func() {
		widget.Card(c).Width(440).Padding(24).Gap(14).Children(func() {
			ui.Row(c).Size(44, 44).Radius(12).Center().Background(theme.AccentSoft).Children(func() {
				ui.Icon(c, svg).FontSize(22).TextColor(theme.Accent)
			})
			ui.Text(c, title).FontSize(17).FontWeight(650).TextColor(theme.TextStrong)
			ui.Text(c, about).FontSize(12.5).TextColor(theme.TextSoft)
			body()
		})
	})
}

func (v *View) errorLine(c *ui.Context) {
	if v.err != "" {
		ui.Text(c, v.err).FontSize(12).TextColor(theme.Error)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// change is the Change master password dialog.
type change struct {
	open                 bool
	old, password, again string
	err                  string
}

func (v *View) changeDialog(c *ui.Context, o Options) {
	f := v.change
	if f == nil || !f.open {
		v.change = nil
		return
	}
	widget.FormSheet(c, &f.open, 440, "Change master password", "The vault's key stays the same: hosts' copies open with the new password once they have the vault again.", func() {
		widget.Field(c, "Current password", func() { widget.Input(c, &f.old, "").FillWidth().Password().AutoFocus() })
		widget.Field(c, "New password", func() { widget.Input(c, &f.password, "At least 8 characters").FillWidth().Password() })
		widget.Field(c, "Again", func() { widget.Input(c, &f.again, "").FillWidth().Password() })
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12).TextColor(theme.Error)
		}
	}, func() {
		if widget.PrimaryButton(c, icon.Check, "Change it", "").Clicked() {
			switch {
			case f.password != f.again:
				f.err = "The two new passwords differ."
			default:
				if f.err = errText(o.Actions.ChangePassword(f.old, f.password)); f.err == "" {
					v.change = nil
				}
			}
		}
	})
}

// Ask is a question the vault puts to the user: a host wants a
// credential, an agent wants to add or remove one, or the vault must be
// unlocked to answer.
type Ask struct {
	Title, About string
	Host         string // the host's name, for "Always on HOST"
	Locked       bool   // the vault must be unlocked first
	Always       bool   // offer Always on Host
	Danger       bool
}

// Answers to an Ask.
const (
	AnswerNone = iota
	AnswerDeny
	AnswerOnce
	AnswerAlways
)

// AskDialog shows ask while *open; it returns the user's answer, unlocking
// the vault first with unlock when it is locked.
func (v *View) AskDialog(c *ui.Context, open *bool, ask Ask, unlock func(password string) error) int {
	answer := AnswerNone
	widget.Dialog(c, open, 460, 14, func() {
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Row(c).Size(36, 36).Shrink(0).Radius(10).Center().Background(theme.AccentSoft).Children(func() {
				ui.Icon(c, icon.Key).FontSize(18).TextColor(theme.Accent)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
				ui.Text(c, ask.Title).FontSize(15).FontWeight(600).TextColor(theme.TextStrong)
				ui.Text(c, ask.About).FontSize(12.5).TextColor(theme.TextSoft)
			})
		})
		if ask.Locked {
			submit := false
			widget.Field(c, "Master password", func() {
				submit = widget.Input(c, &v.password, "Unlock the vault to answer").FillWidth().Password().AutoFocus().Submitted()
			})
			v.errorLine(c)
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Spacer(c)
				if widget.Button(c, widget.Secondary, nil, "Refuse", "").Clicked() {
					answer = AnswerDeny
				}
				if widget.PrimaryButton(c, icon.Lock, "Unlock and answer", "").Clicked() || submit {
					v.err = errText(unlock(v.password))
					if v.err == "" {
						v.password = ""
					}
				}
			})
			return
		}
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Spacer(c)
			if widget.Button(c, widget.Secondary, nil, "Refuse", "").Clicked() {
				answer = AnswerDeny
			}
			if ask.Always {
				if widget.Button(c, widget.Secondary, nil, "Allow once", "").Clicked() {
					answer = AnswerOnce
				}
				if widget.PrimaryButton(c, icon.Check, "Always on "+ask.Host, "").Clicked() {
					answer = AnswerAlways
				}
				return
			}
			label := "Allow"
			if ask.Danger {
				if widget.DangerButton(c, "Remove").Clicked() {
					answer = AnswerOnce
				}
				return
			}
			if widget.PrimaryButton(c, icon.Check, label, "").Clicked() {
				answer = AnswerOnce
			}
		})
	})
	if !*open && answer == AnswerNone {
		answer = AnswerDeny // closed: refused
	}
	return answer
}
