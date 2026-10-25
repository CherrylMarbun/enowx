package ui

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/vps"
	"github.com/enowx/enowx/internal/quietcmd"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Create a server makes a server at a cloud provider with the user's own
// API token (core/vps), with their SSH key for root, then adds it to the
// hosts and connects, which installs enowX there. The token is kept in the
// system's credential store, by provider.

// vpsForm is the Create a server dialog. It belongs to the main thread;
// what it asks the provider answers through the window's Update.
type vpsForm struct {
	open     bool
	provider string
	token    string
	remember bool
	loaded   string // the provider whose regions show
	regions  []vps.Option
	region   string
	sizes    []vps.Option
	size     string
	name     string
	key      string // the public key's file
	busy     string // what is under way, "" for nothing
	err      string
}

func tokenKey(provider string) string { return "vps-token:" + provider }

// Where providers and servers are reached: replaced in tests.
var (
	vpsAPI  = ""   // a provider's API address, "" for its own
	sshPort = "22" // the port a new server's SSH answers on
)

// openCreateServer opens the dialog on the first provider, with its token
// from the credential store when there is one.
func (a *App) openCreateServer() {
	f := &vpsForm{open: true, provider: vps.Providers[0].ID, remember: true, name: "enowx-" + time.Now().Format("0102-1504")}
	f.key = publicKeyFile()
	a.creating = f
	a.loadToken(f)
}

func (a *App) loadToken(f *vpsForm) {
	provider := f.provider
	go func() {
		tok, err := mygo.Secrets.Get(tokenKey(provider))
		a.onMain(func() {
			if err == nil && f.provider == provider && f.token == "" {
				f.token = string(tok)
				a.loadRegions(f)
			}
		})
	}()
}

// publicKeyFile is the user's SSH public key, "" when they have none.
func publicKeyFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
		p := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (a *App) vpsClient(f *vpsForm) (vps.Client, error) { return vps.New(f.provider, f.token, vpsAPI) }

func (a *App) loadRegions(f *vpsForm) {
	c, err := a.vpsClient(f)
	if err != nil {
		f.err = err.Error()
		return
	}
	f.busy, f.err = "Reading regions…", ""
	provider := f.provider
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		regions, err := c.Regions(ctx)
		a.onMain(func() {
			if f.provider != provider {
				return
			}
			f.busy = ""
			if err != nil {
				f.err = err.Error()
				return
			}
			f.regions, f.loaded, f.sizes, f.size = regions, provider, nil, ""
			if len(regions) > 0 {
				f.region = regions[0].ID
				a.loadSizes(f)
			}
		})
	}()
}

func (a *App) loadSizes(f *vpsForm) {
	c, err := a.vpsClient(f)
	if err != nil {
		f.err = err.Error()
		return
	}
	region := f.region
	f.busy = "Reading sizes…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sizes, err := c.Sizes(ctx, region)
		a.onMain(func() {
			if f.region != region {
				return
			}
			f.busy = ""
			if err != nil {
				f.err = err.Error()
				return
			}
			f.sizes, f.size = sizes, ""
			if len(sizes) > 0 {
				f.size = sizes[0].ID
			}
		})
	}()
}

func (a *App) createServerDialog(c *ui.Context) {
	f := a.creating
	if f == nil {
		return
	}
	if !f.open {
		a.creating = nil
		return
	}
	creating := strings.HasPrefix(f.busy, "Creating") || strings.HasPrefix(f.busy, "Waiting") || strings.HasPrefix(f.busy, "Adding")
	widget.Dialog(c, &f.open, 560, 14, func() {
		widget.DialogHeader(c, "Create a server", "At a cloud provider, with your own account and API token: an Ubuntu 24.04 server with your SSH key, which enowX then sets up and connects to. The provider bills you for it.", false)
		if f.err != "" {
			widget.Notice(c, icon.Alert, theme.Error, f.err, "", nil)
		}
		widget.Field(c, "Provider", func() {
			names := make([]string, len(vps.Providers))
			cur := 0
			for i, p := range vps.Providers {
				names[i] = p.Name
				if p.ID == f.provider {
					cur = i
				}
			}
			if i := widget.Segmented(c, names, cur, nil); i >= 0 && vps.Providers[i].ID != f.provider {
				*f = vpsForm{open: true, provider: vps.Providers[i].ID, remember: f.remember, name: f.name, key: f.key}
				a.loadToken(f)
			}
		})
		p, _ := vps.Find(f.provider)
		widget.Field(c, "API token", func() {
			ui.Column(c).FillWidth().Gap(5).Children(func() {
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					widget.Input(c, &f.token, "Paste the token").Password().Grow(1)
					if widget.GhostButton(c, "Use it").Disabled(strings.TrimSpace(f.token) == "" || f.busy != "").Clicked() {
						a.loadRegions(f)
					}
				})
				ui.Text(c, p.TokenHelp+".").FontSize(11.5).TextColor(theme.Muted)
				switchRow(c, &f.remember, "Keep it in the system's keychain", "To create more servers, and to delete this one from enowX.")
			})
		})
		if f.loaded == f.provider && len(f.regions) > 0 {
			widget.Field(c, "Region", func() {
				prev := f.region
				optionSelect(c, &f.region, f.regions, "Region")
				if f.region != prev {
					a.loadSizes(f)
				}
			})
			widget.Field(c, "Size", func() {
				if len(f.sizes) == 0 {
					ui.Text(c, "No sizes offered there.").FontSize(12).TextColor(theme.Muted)
					return
				}
				optionSelect(c, &f.size, f.sizes, "Size")
			})
			widget.Field(c, "Name", func() { widget.Input(c, &f.name, "enowx-1").FillWidth().Font(theme.MonoFont) })
			widget.Field(c, "SSH key", func() {
				if f.key != "" {
					ui.Text(c, f.key+", for root").Font(theme.MonoFont).FontSize(11.5).TextColor(theme.TextSoft)
					return
				}
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					ui.Text(c, "You have no SSH key yet.").FontSize(12).TextColor(theme.Muted).Grow(1)
					if widget.GhostButton(c, "Make one").Clicked() {
						a.makeKey(f)
					}
				})
			})
		}
		widget.DialogFooter(c, f.busy, func() {
			if widget.GhostButton(c, "Cancel").Disabled(creating).Clicked() && !creating {
				f.open = false
			}
			ok := f.size != "" && f.key != "" && strings.TrimSpace(f.name) != "" && f.busy == ""
			if widget.PrimaryButton(c, icon.Server, "Create", "").Disabled(!ok).Clicked() && ok {
				a.createServer(f)
			}
		})
	})
	if creating {
		f.open = true // it finishes, or fails, before the dialog goes
	}
}

// optionSelect is a drop-down of provider options, with prices.
func optionSelect(c *ui.Context, value *string, opts []vps.Option, label string) {
	sel := ui.SelectBase(c, value)
	cur := vps.Option{Name: *value}
	for _, o := range opts {
		if o.ID == *value {
			cur = o
		}
	}
	widget.Trigger(sel.Trigger, sel.Open()).FillWidth().Label(label).Children(func() {
		ui.Text(c, cur.Name).FontSize(12.5).SingleLine()
		ui.Text(c, optionNote(cur)).FontSize(11.5).TextColor(theme.Muted).SingleLine().Shrink(1)
		ui.Spacer(c)
		ui.Icon(c, icon.ChevronDown).FontSize(12).TextColor(theme.Muted)
	})
	sel.Popup(func(panel ui.Element) {
		widget.Popup(panel.Margin(6, 0, 0, 0).Width(480))
		ui.Scroll(c).FillWidth().MaxHeight(320).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for _, o := range opts {
					item := sel.Item(o.ID)
					widget.PopupItem(item, item.Highlighted()).Children(func() {
						ui.Text(c, o.Name).FontSize(12.5).SingleLine()
						ui.Text(c, optionNote(o)).FontSize(11.5).TextColor(theme.Muted).SingleLine().Shrink(1)
					})
				}
			})
		})
	})
}

func optionNote(o vps.Option) string {
	note := o.Detail
	if o.Monthly > 0 {
		note += fmt.Sprintf(" · %.2f a month", o.Monthly)
	}
	return note
}

// makeKey makes the user an ed25519 SSH key with ssh-keygen, no passphrase,
// where they have none.
func (a *App) makeKey(f *vpsForm) {
	home, err := os.UserHomeDir()
	if err != nil {
		f.err = err.Error()
		return
	}
	key := filepath.Join(home, ".ssh", "id_ed25519")
	f.busy = "Making an SSH key…"
	go func() {
		_ = os.MkdirAll(filepath.Dir(key), 0o700)
		out, err := quietcmd.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", "enowX", "-f", key).CombinedOutput()
		a.onMain(func() {
			f.busy = ""
			if err != nil {
				f.err = "ssh-keygen: " + strings.TrimSpace(string(out))
				return
			}
			f.key = key + ".pub"
		})
	}()
}

// createServer creates the server, waits until it is up and its SSH
// answers, then adds it to the hosts and connects.
func (a *App) createServer(f *vpsForm) {
	c, err := a.vpsClient(f)
	if err != nil {
		f.err = err.Error()
		return
	}
	pub, err := os.ReadFile(f.key)
	if err != nil {
		f.err = err.Error()
		return
	}
	if f.remember {
		_ = mygo.Secrets.Set(tokenKey(f.provider), []byte(strings.TrimSpace(f.token)))
	}
	f.busy, f.err = "Creating the server…", ""
	p := vps.CreateParams{Name: strings.TrimSpace(f.name), Region: f.region, Size: f.size, PublicKey: string(pub)}
	provider := f.provider
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		s, err := c.Create(ctx, p)
		if err == nil {
			s, err = vps.WaitReady(ctx, c, s.ID, 3*time.Second, func(s vps.Server) {
				a.onMain(func() { f.busy = "Waiting for it to start (" + s.Status + ")…" })
			})
		}
		if err == nil {
			a.onMain(func() { f.busy = "Waiting for its SSH to answer…" })
			err = waitSSH(ctx, s.IPv4)
		}
		a.onMain(func() {
			if err != nil {
				f.busy, f.err = "", err.Error()
				return
			}
			f.busy = "Adding it to your hosts…"
			h, err := a.opts.Hosts.Add(hosts.Host{Name: p.Name, Address: "root@" + s.IPv4, Provider: provider, ProviderID: s.ID})
			f.busy = ""
			if err != nil {
				f.err = err.Error()
				return
			}
			f.open = false
			a.reloadHosts()
			a.page = ""
			a.connect(h)
		})
	}()
}

// waitSSH waits until the server's SSH port takes connections.
func waitSSH(ctx context.Context, ip string) error {
	for {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, sshPort), 5*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the server's SSH never answered: %w", err)
		case <-time.After(3 * time.Second):
		}
	}
}

// deleteServerDialog asks before deleting a server at its provider, which
// can't be undone.
func (a *App) deleteServerDialog(c *ui.Context) {
	h := a.deletingServer
	if h == nil {
		return
	}
	p, _ := vps.Find(h.Provider)
	open := true
	msg := "Its disk and everything on it go for good, sessions included, and " + p.Name + " stops billing for it. enowX removes it from your hosts."
	switch ui.AlertDialog(c, &open, "Delete "+h.Name+" at "+p.Name+"?", msg, "Cancel", "Delete the server") {
	case 1:
		a.deleteServer(*h)
		a.deletingServer = nil
	case 0, ui.AlertDismissed:
		a.deletingServer = nil
	}
	if !open {
		a.deletingServer = nil
	}
}

// deleteServer deletes host h's server at its provider, then removes it
// from the hosts. The provider's token comes from the keychain.
func (a *App) deleteServer(h hosts.Host) {
	go func() {
		tok, err := mygo.Secrets.Get(tokenKey(h.Provider))
		if err == nil {
			var c vps.Client
			if c, err = vps.New(h.Provider, string(tok), vpsAPI); err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				err = c.Delete(ctx, h.ProviderID)
				cancel()
			}
		} else {
			err = fmt.Errorf("no token for %s in the keychain: delete the server in its console", h.Provider)
		}
		a.onMain(func() {
			if err != nil {
				a.status = err.Error()
				return
			}
			_ = a.opts.Hosts.Remove(h.ID)
			a.reloadHosts()
			if a.host.ID == h.ID {
				a.connect(LocalHost())
			}
			a.status = h.Name + " was deleted."
		})
	}()
}
