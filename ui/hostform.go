package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// addHostForm is the state of the Add VPS dialog.
type addHostForm struct {
	open     bool
	manual   bool // show the address form when the tailnet has devices
	name     string
	address  string
	identity string
	device   bool // one of the user's computers, not a server
	err      string
	// editing is the saved host being edited; its ID is "" when adding.
	editing        hosts.Host
	forgetPassword bool
}

func (a *App) addHostDialog(c *ui.Context) {
	f := &a.addHost
	if !f.open {
		return
	}
	hasTailnet := len(a.peers) > 0 && f.editing.ID == ""
	// With a tailnet, its devices come first; adding by address opens a
	// dialog of its own in their place, which Back returns from.
	byAddress := hasTailnet && f.manual
	key := "tailnet"
	if !hasTailnet || f.manual {
		key = "address"
	}
	widget.Dialog(c.Key(key), &f.open, 520, 16, func() {
		title := "Add VPS"
		crumbs := []string{"Hosts", title}
		desc := "Any Linux server you can reach over SSH, by IP, domain or Tailscale name."
		switch {
		case f.editing.ID != "":
			title = "Edit " + f.editing.Name
			crumbs = []string{"Hosts", title}
		case byAddress:
			title = "Add by address"
			crumbs = []string{"Hosts", "Add VPS", "By address"}
			desc = "A server outside your tailnet, by IP or domain."
		}
		if widget.DialogHeader(c, title, desc, true, crumbs...) {
			f.open = false
		}
		if hasTailnet && !byAddress {
			a.tailnetSection(c)
			b := ui.ButtonBase(c).Gap(8).Cursor(ui.CursorPointer).TextColor(theme.TextSoft).Label("Add by address")
			if b.Hovered() {
				b.TextColor(theme.Accent)
			}
			b.Children(func() {
				ui.Icon(c, icon.Plus).FontSize(13)
				ui.Text(c, "Add a server by address instead").FontSize(12.5)
			})
			if b.Clicked() {
				f.manual = true
			}
			return
		} else if !hasTailnet && a.peersErr != "" && f.editing.ID == "" {
			ui.Text(c, a.peersErr).FontSize(12).TextColor(theme.Muted)
		}
		submit := false
		widget.Field(c, "SSH address", func() {
			in := widget.Input(c, &f.address, "user@203.0.113.10 or user@my-vps:2222").FillWidth().Font(theme.MonoFont).AutoFocus()
			submit = in.Submitted()
		})
		widget.Field(c, "Name (optional)", func() {
			submit = widget.Input(c, &f.name, "vps-sg-1").FillWidth().Submitted() || submit
		})
		widget.Field(c, "Private key (optional)", func() {
			submit = widget.Input(c, &f.identity, "~/.ssh/id_ed25519 · also tries ~/.ssh/config and the SSH agent").FillWidth().Font(theme.MonoFont).Submitted() || submit
		})
		widget.Field(c, "Listed under", func() {
			cur := 0
			if f.device {
				cur = 1
			}
			if i := widget.Segmented(c, []string{"VPS", "Devices"}, cur, nil); i >= 0 {
				f.device = i == 1
			}
		})
		if f.editing.PasswordAuth && !f.forgetPassword {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "A password is saved in the keychain for this host.").FontSize(12).TextColor(theme.TextSoft).Grow(1)
				if widget.GhostButton(c, "Forget password").Clicked() {
					f.forgetPassword = true
				}
			})
		}
		ui.Text(c, "enowX signs in with your SSH keys and asks for a password if the server wants one. It installs its daemon in ~/.enowx on the server; sessions keep running there when you disconnect.").
			FontSize(12).TextColor(theme.Muted)
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12).TextColor(theme.Error)
		}
		widget.DialogFooter(c, "", func() {
			if byAddress && widget.GhostButton(c, "Back to tailnet").Clicked() {
				f.manual, f.err = false, ""
			}
			if widget.GhostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			label := "Add and connect"
			if f.editing.ID != "" {
				label = "Save and connect"
			}
			if widget.PrimaryButton(c, nil, label, "").Clicked() {
				submit = true
			}
		})
		if submit {
			a.saveHost()
		}
	})
}

func (a *App) saveHost() {
	f := &a.addHost
	if a.opts.Hosts == nil {
		f.err = "This build keeps no host list."
		return
	}
	if strings.TrimSpace(f.address) == "" {
		f.err = "Type the server's SSH address."
		return
	}
	h := f.editing // keeps the ID, the run-as choice and a saved password
	h.Name, h.Address, h.IdentityFile = f.name, f.address, strings.TrimSpace(f.identity)
	h.Kind = ""
	if f.device {
		h.Kind = hosts.KindDevice
	}
	if f.forgetPassword {
		h.PasswordAuth = false
	}
	var err error
	if h.ID != "" {
		if err = a.opts.Hosts.Update(h); err == nil {
			h, _ = a.opts.Hosts.Get(h.ID)
			if f.forgetPassword {
				a.forgetPassword(f.editing)
			}
		}
	} else {
		h, err = a.opts.Hosts.Add(h)
	}
	if err != nil {
		f.err = err.Error()
		return
	}
	f.open = false
	a.reloadHosts()
	a.connect(h)
}

// openEditHost opens the host dialog on a saved host.
func (a *App) openEditHost(h hosts.Host) {
	a.addHost = addHostForm{open: true, manual: true, editing: h, name: h.Name, address: h.Address, identity: h.IdentityFile, device: h.Kind == hosts.KindDevice}
}

func (a *App) storedHost(id string) (hosts.Host, bool) {
	if a.opts.Hosts == nil {
		return hosts.Host{}, false
	}
	return a.opts.Hosts.Get(id)
}
