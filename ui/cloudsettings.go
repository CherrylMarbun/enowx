package ui

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Settings › Cloud: the connected host's sign-in to a team cloud
// (docs/cloud.md), through its daemon like everything else: the cloud's
// address, signing in with a code the browser approves, the account's
// devices, and its teams (cloudteams.go).

const sectionCloud = "Cloud"

// defaultCloudURL is offered, not set: the cloud stays off until the user
// turns it on.
const defaultCloudURL = "https://cloud.enowx.ai"

// cloudSettings is the section's state for the connected host, read when
// it shows. It belongs to the main thread.
type cloudSettings struct {
	host    string
	st      proto.CloudStatus
	loaded  bool
	busy    string // what is under way, shown
	err     string
	url     string
	approve string // a code typed, to approve another device
	opened  string // the code whose page was opened, once
	devices []api.Device
	teams   cloudTeams
}

func (a *App) cloudOf() *cloudSettings {
	if a.cloudSet == nil || a.cloudSet.host != a.host.ID {
		a.cloudSet = &cloudSettings{host: a.host.ID, url: defaultCloudURL}
		a.loadCloud()
	}
	return a.cloudSet
}

// loadCloud reads the host's sign-in, and when signed in its devices and
// teams.
func (a *App) loadCloud() {
	cs, cl := a.cloudSet, a.client
	if cs == nil || cl == nil {
		return
	}
	go func() {
		st, err := cl.CloudStatus()
		var devices []api.Device
		var teams []api.Team
		if err == nil && st.SignedIn {
			if e := cl.CloudCall("GET", api.PathDevices, nil, &devices); e != nil {
				err = e
			}
			if e := cl.CloudCall("GET", api.PathTeams, nil, &teams); e != nil && err == nil {
				err = e
			}
		}
		a.onMain(func() {
			cs.loaded = true
			cs.err = ""
			if err != nil {
				cs.err = client.Explain(err)
			}
			cs.setStatus(st)
			cs.devices, cs.teams.list = devices, teams
		})
	}()
}

func (cs *cloudSettings) setStatus(st proto.CloudStatus) {
	cs.st = st
	if st.URL != "" {
		cs.url = st.URL
	}
	if st.Err != "" {
		cs.err = st.Err
	}
}

// cloudEvent takes the host's sign-in as it changes; it runs on the
// client's goroutine.
func (a *App) cloudEvent(ev proto.Event) {
	if ev.Name != proto.EventCloudChanged {
		return
	}
	var st proto.CloudStatus
	if json.Unmarshal(ev.Data, &st) != nil {
		return
	}
	a.onMain(func() {
		cs := a.cloudSet
		if cs == nil || cs.host != a.host.ID {
			return
		}
		was := cs.st.SignedIn
		cs.setStatus(st)
		if st.SignedIn && !was {
			a.loadCloud() // signed in just now: its devices and teams
		}
		if !st.SignedIn {
			cs.devices, cs.teams = nil, cloudTeams{}
		}
	})
}

// cloudDo runs fn off the main thread, then done on it, showing what is
// under way and the error if any.
func (a *App) cloudDo(what string, fn func(cl *client.Client) error, done func()) {
	cs, cl := a.cloudSet, a.client
	if cs == nil || cl == nil || cs.busy != "" {
		return
	}
	cs.busy, cs.err = what, ""
	go func() {
		err := fn(cl)
		a.onMain(func() {
			cs.busy = ""
			if err != nil {
				cs.err = client.Explain(err)
				return
			}
			if done != nil {
				done()
			}
		})
	}()
}

func (a *App) cloudSection(c *ui.Context) {
	cs := a.cloudOf()
	sectionIntro(c, "Cloud", "Your teams in enowX: their members, the notes and canvases they share, their hosts. Off until you set a cloud's address; nothing reaches it before. This is "+a.host.Name+"'s sign-in: each host signs in as a device of your account.")
	if cs.err != "" {
		widget.Notice(c, icon.Alert, theme.Error, cs.err, "", nil)
	}
	if !cs.loaded {
		ui.Text(c, "Asking "+a.host.Name+"…").FontSize(12).TextColor(theme.Muted)
		return
	}
	a.cloudServer(c, cs)
	if cs.st.URL == "" {
		return
	}
	a.cloudAccount(c, cs)
	if !cs.st.SignedIn {
		return
	}
	a.cloudDevices(c, cs)
	a.cloudTeamsSection(c, cs)
}

// cloudServer is the cloud's address: set, it is on.
func (a *App) cloudServer(c *ui.Context, cs *cloudSettings) {
	widget.Field(c, "Cloud", func() {
		if cs.st.URL != "" {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Padding(10, 12).Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border).Children(func() {
				ui.Icon(c, icon.Globe).FontSize(14).TextColor(theme.TextSoft)
				ui.Text(c, cs.st.URL).FontSize(12.5).Grow(1).Shrink(1).MinWidth(0).SingleLine()
				if widget.GhostButton(c, "Turn off").Tooltip("Sign this host out, and stop using the cloud").Clicked() {
					a.cloudDo("Turning the cloud off…", func(cl *client.Client) error {
						st, err := cl.SetCloudURL("")
						a.onMain(func() { cs.setStatus(st) })
						return err
					}, nil)
				}
			})
			return
		}
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() { widget.Input(c, &cs.url, defaultCloudURL).FillWidth() })
			if widget.PrimaryButton(c, icon.Globe, "Turn on", "").Clicked() && strings.TrimSpace(cs.url) != "" {
				url := strings.TrimSpace(cs.url)
				a.cloudDo("Reaching "+url+"…", func(cl *client.Client) error {
					st, err := cl.SetCloudURL(url)
					a.onMain(func() { cs.setStatus(st) })
					return err
				}, nil)
			}
		})
		ui.Text(c, "enowX's own is "+defaultCloudURL+"; a team can run its own (docs/cloud.md).").FontSize(11.5).TextColor(theme.Muted)
	})
	if cs.busy != "" {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			widget.Spinner(c, 14)
			ui.Text(c, cs.busy).FontSize(12).TextColor(theme.Muted)
		})
	}
}

// cloudAccount is who the host is signed in as, or signing in.
func (a *App) cloudAccount(c *ui.Context, cs *cloudSettings) {
	widget.Field(c, "Account", func() {
		switch {
		case cs.st.SignedIn:
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Padding(12).Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border).Children(func() {
				initial := "?"
				if cs.st.Account != "" {
					initial = strings.ToUpper(string([]rune(cs.st.Account)[0]))
				}
				ui.Column(c).Width(32).Height(32).Radius(16).Background(theme.Hover).Center().Children(func() {
					ui.Text(c, initial).FontSize(14).FontWeight(600)
				})
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(2).Children(func() {
					who := cs.st.Account
					if cs.st.GitHub != "" {
						who += " · @" + cs.st.GitHub
					}
					ui.Text(c, who).FontSize(13).FontWeight(600).SingleLine()
					device := a.host.Name + ", signed in as " + deviceKind(cs.st.DeviceKind)
					if cs.st.DeviceName != a.host.Name {
						device = a.host.Name + " is " + cs.st.DeviceName + " (" + deviceKind(cs.st.DeviceKind) + ")"
					}
					ui.Text(c, device).FontSize(11.5).TextColor(theme.Muted).SingleLine()
				})
				if widget.GhostButton(c, "Sign out").Clicked() {
					a.cloudDo("Signing out…", func(cl *client.Client) error {
						st, err := cl.CloudSignOut()
						a.onMain(func() { cs.setStatus(st); cs.devices, cs.teams = nil, cloudTeams{} })
						return err
					}, nil)
				}
			})
		case cs.st.Pending != nil:
			p := cs.st.Pending
			if p.VerifyURLComplete != "" && cs.opened != p.UserCode {
				cs.opened = p.UserCode // the browser opens once per code
				url := p.VerifyURLComplete
				go func() { _ = mygo.Shell.OpenExternal(url) }()
			}
			ui.Column(c).FillWidth().Gap(10).Padding(16).Radius(theme.RadiusControl).Background(theme.Sunken).Border(1, theme.Border).Children(func() {
				ui.Text(c, "Approve this code in your browser, signed in with GitHub:").FontSize(12.5).TextColor(theme.TextSoft)
				ui.Text(c, p.UserCode).Font(theme.MonoFont).FontSize(26).FontWeight(600)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					widget.Spinner(c, 12)
					ui.Text(c, "Waiting, until "+p.Expires.Local().Format("15:04")+"…").FontSize(11.5).TextColor(theme.Muted).Grow(1)
					if p.VerifyURLComplete != "" && widget.Button(c, widget.Secondary, icon.External, "Open the page again", "").Clicked() {
						url := p.VerifyURLComplete
						go func() { _ = mygo.Shell.OpenExternal(url) }()
					}
					if widget.GhostButton(c, "Cancel").Clicked() {
						a.cloudDo("", func(cl *client.Client) error {
							st, err := cl.CloudCancel()
							a.onMain(func() { cs.setStatus(st) })
							return err
						}, nil)
					}
				})
				c.After(time.Second) // the time left moves
			})
		default:
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Not signed in. Sign in with GitHub: the browser opens on a code to approve.").FontSize(12.5).TextColor(theme.TextSoft).Grow(1).Shrink(1)
				if widget.PrimaryButton(c, icon.Key, "Sign in", "").Clicked() {
					kind := proto.CloudDeviceHost
					if a.host.ID == hosts.LocalID {
						kind = proto.CloudDeviceApp
					}
					a.cloudDo("Asking for a code…", func(cl *client.Client) error {
						st, err := cl.CloudSignIn(proto.CloudSignInParams{Kind: kind, Name: a.host.Name})
						a.onMain(func() { cs.setStatus(st) })
						return err
					}, nil)
				}
			})
		}
	})
}

func deviceKind(kind string) string {
	if kind == proto.CloudDeviceHost {
		return "a host"
	}
	return "the app"
}

// cloudDevices are the account's devices, to sign out, and a code to
// approve for another.
func (a *App) cloudDevices(c *ui.Context, cs *cloudSettings) {
	widget.Field(c, "Devices", func() {
		list := append([]api.Device(nil), cs.devices...)
		sort.SliceStable(list, func(i, j int) bool { return list[i].Current && !list[j].Current })
		ui.Column(c).FillWidth().Gap(4).Children(func() {
			for _, d := range list {
				ui.Row(c.Key("dev:"+d.ID)).FillWidth().Gap(10).AlignItems(ui.Center).Padding(8, 10).Radius(theme.RadiusControl).Background(theme.Sunken).Children(func() {
					svg := icon.Laptop
					if d.Kind == api.DeviceHost {
						svg = icon.Server
					}
					ui.Icon(c, svg).FontSize(14).TextColor(theme.TextSoft)
					ui.Text(c, d.Name).FontSize(12.5).SingleLine().Shrink(1)
					if d.Current {
						widget.Tag(c, "this host", theme.Accent)
					}
					ui.Spacer(c)
					ui.Text(c, "seen "+textfmt.Age(d.LastSeen, time.Now())).FontSize(11).TextColor(theme.Muted)
					if !d.Current && widget.IconButton(c, icon.Close, "Sign "+d.Name+" out", false).Clicked() {
						id := d.ID
						a.cloudDo("Signing "+d.Name+" out…", func(cl *client.Client) error {
							return cl.CloudCall("DELETE", strings.Replace(api.PathDevice, "{id}", id, 1), nil, nil)
						}, a.loadCloud)
					}
				})
			}
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				widget.Input(c, &cs.approve, "Code another device shows (XXXX-XXXX)").FillWidth().Font(theme.MonoFont)
			})
			if widget.Button(c, widget.Secondary, icon.Check, "Approve", "").Clicked() && strings.TrimSpace(cs.approve) != "" {
				code := strings.TrimSpace(cs.approve)
				a.cloudDo("Approving "+code+"…", func(cl *client.Client) error {
					return cl.CloudCall("POST", api.PathDeviceApprove, api.DeviceApproveParams{UserCode: code}, nil)
				}, func() { cs.approve = ""; a.loadCloud() })
			}
		})
	})
}
