package ui

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/transport"
)

// Pseudo-host IDs for the switcher's actions.
const (
	actionAddHost     = "+add"
	actionManageHosts = "+manage"
	actionSync        = "+sync"
	tailscalePrefix   = "+ts:"
)

// hostSwitcher is the card at the top of the sidebar: the current host,
// opening a list of hosts to switch to.
func (a *App) hostSwitcher(c *ui.Context) {
	choice := a.host.ID
	sel := ui.SelectBase(c, &choice)
	trig := sel.Trigger.FillWidth().Gap(10).Padding(8, 10).Radius(10).Background(colRaised).
		Border(1, colBorderHi).Cursor(ui.CursorPointer).Label("Host: " + a.host.Name)
	if sel.Open() {
		trig.Border(1, colAccent)
	}
	trig.Children(func() {
		hostTile(c, a.host)
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, a.host.Name).Bold().SingleLine()
			ui.Row(c).Gap(5).Children(func() {
				col, label := a.connectionState()
				dot(c, col, false, 6)
				ui.Text(c, label).FontSize(11).TextColor(colMuted).SingleLine()
			})
		})
		ui.Icon(c, iconChevronsUD).FontSize(14).TextColor(colMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(6, 0, 0, 0).Padding(6).Radius(12).Width(300).Background(ui.Hex("#1B1E25")).
			Border(1, ui.Hex("#2E333C")).Shadow(0, 18, 40, 0, ui.RGBA(0, 0, 0, 0.55))
		ui.Text(c, "HOSTS").FontSize(11).Bold().TextColor(colMuted).Padding(6, 8, 4, 8)
		for _, h := range a.allHosts() {
			item := sel.Item(h.ID).FillWidth().Gap(10).Padding(8, 8).Radius(8)
			if item.Highlighted() {
				item.Background(ui.Hex("#262B35"))
			}
			item.Children(func() {
				hostTile(c, h)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, h.Name).Bold().SingleLine()
					ui.Text(c, hostDetail(h)).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
				})
				if h.ID == a.host.ID {
					ui.Icon(c, iconCheck).FontSize(15).TextColor(colAccent)
				}
			})
		}
		ui.Text(c, "TAILSCALE").FontSize(11).Bold().TextColor(colMuted).Padding(10, 8, 4, 8)
		if note := a.tailnetNote(); note != "" {
			ui.Text(c, note).FontSize(12).TextColor(colMuted).Padding(2, 8, 6, 8).MaxWidth(288)
		}
		if peers := a.newPeers(); len(peers) > 0 {
			for _, p := range peers {
				item := sel.Item(tailscalePrefix+p.Address()).FillWidth().Gap(10).Padding(8, 8).Radius(8)
				if item.Highlighted() {
					item.Background(ui.Hex("#262B35"))
				}
				item.Children(func() {
					ui.Box(c).Size(28, 28).Radius(7).Background(colSelected).Center().TextColor(colTextSoft).Children(func() {
						ui.Icon(c, iconTailscale).FontSize(14)
					})
					ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, p.HostName).Bold().SingleLine()
						ui.Text(c, p.OS+" · "+p.Address()).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
					})
					if p.Online {
						dot(c, colDone, false, 7)
					} else {
						dot(c, colIdle, true, 7)
					}
				})
			}
		}
		ui.Box(c).Height(1).FillWidth().Margin(4, 6).Background(colBorderHi)
		type action struct {
			id, label string
			icon      *ui.SVG
		}
		actions := []action{{actionAddHost, "Add VPS…", iconPlus}, {actionManageHosts, "Manage hosts…", iconServer}}
		if a.client != nil && a.host.ID != hosts.LocalID {
			actions = append([]action{{actionSync, "Sync environment…", iconSync}}, actions...)
		}
		for _, action := range actions {
			item := sel.Item(action.id).FillWidth().Gap(10).Padding(8, 8).Radius(8).TextColor(colTextSoft)
			if item.Highlighted() {
				item.Background(ui.Hex("#262B35"))
			}
			item.Children(func() {
				ui.Box(c).Size(28, 28).Center().Children(func() { ui.Icon(c, action.icon).FontSize(14) })
				ui.Text(c, action.label)
			})
		}
	})
	if sel.Open() && !a.peersAsked {
		a.peersAsked = true
		go a.loadPeers()
	} else if !sel.Open() {
		a.peersAsked = false
	}
	if choice == a.host.ID {
		return
	}
	if addr, ok := strings.CutPrefix(choice, tailscalePrefix); ok {
		for _, p := range a.peers {
			if p.Address() == addr {
				a.connectPeer(p)
			}
		}
		return
	}
	switch choice {
	case actionAddHost:
		a.openAddHost()
	case actionManageHosts:
		a.manageHosts = true
	case actionSync:
		a.openSync()
	default:
		for _, h := range a.allHosts() {
			if h.ID == choice {
				a.connect(h)
			}
		}
	}
}

func (a *App) connectionState() (ui.Color, string) {
	switch {
	case a.connecting != "":
		return ui.Hex("#F2C15B"), "Connecting…"
	case a.client == nil:
		return colError, "Not connected"
	case a.host.ID == hosts.LocalID:
		return colDone, "Local"
	}
	if a.host.RunAs != "" {
		return colDone, "SSH · " + a.host.Address + " · as " + a.host.RunAs
	}
	return colDone, "SSH · " + a.host.Address
}

func hostTile(c *ui.Context, h hosts.Host) {
	ui.Box(c).Size(28, 28).Radius(7).Background(colSelected).Center().TextColor(colTextSoft).Children(func() {
		if h.ID == hosts.LocalID {
			ui.Icon(c, iconLaptop).FontSize(14)
		} else {
			ui.Icon(c, iconServer).FontSize(14)
		}
	})
}

func hostDetail(h hosts.Host) string {
	if h.ID == hosts.LocalID {
		return "this machine"
	}
	return h.Address
}

// connectionView fills the workspace while connecting or after a failure.
func (a *App) connectionView(c *ui.Context) bool {
	switch {
	case a.connecting != "":
		ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Children(func() {
			spin := ui.Box(c).Size(22, 22).Radius(11).Border(2.5, ui.Hex("#2E3B55"))
			spin.Children(func() {
				ui.Box(c).Size(22, 22).Radius(11).BorderWidth(2.5, 0, 0, 0).BorderColor(colAccent).Margin(-2.5)
			})
			ui.Text(c, a.connecting).TextColor(colTextSoft)
		})
		return true
	case a.connErr != "":
		ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
			ui.Text(c, "Couldn't connect to "+a.host.Name).FontSize(16).Bold()
			ui.Text(c, a.connErr).TextColor(colMuted).MaxWidth(520).TextAlign(ui.Center)
			ui.Row(c).Gap(8).Margin(6, 0, 0, 0).Children(func() {
				if primaryButton(c, iconRestore, "Retry", "").Clicked() {
					a.connect(a.host)
				}
				var refusal *transport.AuthError
				if errors.As(a.connFail, &refusal) && ghostButton(c, "Sign in…").Clicked() {
					a.promptSignIn(a.host, a.connFail)
				}
				if _, saved := a.storedHost(a.host.ID); saved && ghostButton(c, "Edit host…").Clicked() {
					a.openEditHost(a.host)
				}
				if a.host.ID != hosts.LocalID && ghostButton(c, "Back to "+LocalHost().Name).Clicked() {
					a.connect(LocalHost())
				}
			})
		})
		return true
	}
	return false
}

// addHostForm is the state of the Add VPS dialog.
type addHostForm struct {
	open     bool
	manual   bool // show the address form when the tailnet has devices
	name     string
	address  string
	identity string
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
	ui.DialogBase(c, &f.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(520).Padding(20).Gap(16).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi).
			Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.55))
		ui.Row(c).FillWidth().Children(func() {
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				title := "Add VPS"
				if f.editing.ID != "" {
					title = "Edit " + f.editing.Name
				}
				ui.Text(c, title).FontSize(16).Bold()
				ui.Text(c, "Any Linux server you can reach over SSH, by IP, domain or Tailscale name.").FontSize(12).TextColor(colMuted)
			})
			if iconButton(c, iconClose, "Close", false).Clicked() {
				f.open = false
			}
		})
		hasTailnet := len(a.peers) > 0 && f.editing.ID == ""
		if hasTailnet {
			a.tailnetSection(c)
			if !f.manual {
				b := ui.ButtonBase(c).Gap(8).Cursor(ui.CursorPointer).TextColor(colTextSoft).Label("Add by address")
				b.Children(func() {
					ui.Icon(c, iconPlus).FontSize(13)
					ui.Text(c, "Add a server by address instead").FontSize(13)
				})
				if b.Clicked() {
					f.manual = true
				}
				return
			}
			ui.Box(c).Height(1).FillWidth().Background(colBorderHi)
		} else if a.peersErr != "" && f.editing.ID == "" {
			ui.Text(c, a.peersErr).FontSize(12).TextColor(colMuted)
		}
		submit := false
		field(c, "SSH address", func() {
			in := ui.TextInput(c, &f.address).Font(monoFont).Placeholder("user@203.0.113.10 or user@my-vps:2222")
			if !hasTailnet {
				in.AutoFocus()
			}
			submit = in.Submitted()
		})
		field(c, "Name (optional)", func() {
			submit = ui.TextInput(c, &f.name).Placeholder("vps-sg-1").Submitted() || submit
		})
		field(c, "Private key (optional)", func() {
			submit = ui.TextInput(c, &f.identity).Font(monoFont).Placeholder("~/.ssh/id_ed25519 · also tries ~/.ssh/config and the SSH agent").Submitted() || submit
		})
		if f.editing.PasswordAuth && !f.forgetPassword {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "A password is saved in the keychain for this host.").FontSize(12).TextColor(colTextSoft).Grow(1)
				if ghostButton(c, "Forget password").Clicked() {
					f.forgetPassword = true
				}
			})
		}
		ui.Text(c, "enowX signs in with your SSH keys and asks for a password if the server wants one. It installs its daemon in ~/.enowx on the server; sessions keep running there when you disconnect.").
			FontSize(12).TextColor(colMuted)
		if f.err != "" {
			ui.Text(c, f.err).FontSize(12).TextColor(colError)
		}
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if ghostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			label := "Add and connect"
			if f.editing.ID != "" {
				label = "Save and connect"
			}
			if primaryButton(c, iconServer, label, "").Clicked() {
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

func (a *App) manageHostsDialog(c *ui.Context) {
	if !a.manageHosts {
		return
	}
	ui.DialogBase(c, &a.manageHosts, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(520).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi)
		ui.Row(c).FillWidth().Children(func() {
			ui.Text(c, "Hosts").FontSize(16).Bold().Grow(1)
			if iconButton(c, iconClose, "Close", false).Clicked() {
				a.manageHosts = false
			}
		})
		ui.Text(c, "Saved on this device in ~/.enowx/hosts.json. Removing a host leaves its server and sessions untouched.").
			FontSize(12).TextColor(colMuted)
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			if len(a.hostList) == 0 {
				ui.Text(c, "No servers yet.").TextColor(colMuted).Padding(8, 0)
			}
			for _, h := range a.hostList {
				ui.Row(c.Key(h.ID)).FillWidth().Gap(10).Padding(8, 10).Radius(8).Background(colRaised).Children(func() {
					hostTile(c, h)
					ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, h.Name).Bold().SingleLine()
						ui.Text(c, h.Address).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
						if note := runAsNote(h); note != "" {
							ui.Text(c, note).FontSize(11).TextColor(colTextSoft).SingleLine()
						}
					})
					if ghostButton(c, "Edit…").Clicked() {
						a.manageHosts = false
						a.openEditHost(h)
					}
					if (h.RunAs != "" || h.KeepRoot) && ghostButton(c, "Ask user again").Clicked() {
						h.RunAs, h.KeepRoot = "", false
						if err := a.opts.Hosts.Update(h); err != nil {
							a.status = err.Error()
						}
						a.reloadHosts()
					}
					if ghostButton(c, "Remove").Clicked() {
						if err := a.opts.Hosts.Remove(h.ID); err != nil {
							a.status = err.Error()
						}
						a.forgetPassword(h)
						a.reloadHosts()
						if h.ID == a.host.ID {
							a.connect(LocalHost())
						}
					}
				})
			}
		})
		ui.Row(c).FillWidth().Justify(ui.End).Children(func() {
			if primaryButton(c, iconPlus, "Add VPS…", "").Clicked() {
				a.manageHosts = false
				a.openAddHost()
			}
		})
	})
}

func (a *App) trustDialog(c *ui.Context) {
	if a.trust == nil {
		return
	}
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(520).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi)
		ui.Text(c, "Trust this server?").FontSize(16).Bold()
		ui.Text(c, "This is the first connection to "+a.trust.addr+". Check that its key fingerprint matches the server's before trusting it:").
			FontSize(13).TextColor(colTextSoft)
		ui.Text(c, a.trust.fingerprint).Font(monoFont).FontSize(12).Padding(10).Radius(8).Background(colBackground).FillWidth()
		ui.Text(c, "On the server: for f in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf $f; done").Font(monoFont).FontSize(11).TextColor(colMuted)
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if ghostButton(c, "Cancel").Clicked() {
				a.answerTrust(false)
			}
			if primaryButton(c, iconCheck, "Trust and connect", "").Clicked() {
				a.answerTrust(true)
			}
		})
	})
	if !open {
		a.answerTrust(false)
	}
}

// openEditHost opens the host dialog on a saved host.
func (a *App) openEditHost(h hosts.Host) {
	a.addHost = addHostForm{open: true, manual: true, editing: h, name: h.Name, address: h.Address, identity: h.IdentityFile}
}

func (a *App) storedHost(id string) (hosts.Host, bool) {
	if a.opts.Hosts == nil {
		return hosts.Host{}, false
	}
	return a.opts.Hosts.Get(id)
}
