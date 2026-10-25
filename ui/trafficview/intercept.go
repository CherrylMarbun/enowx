package trafficview

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// interceptState is the Intercept page: what is being attached, and the
// CA when asked for.
type interceptState struct {
	busy     string // the target being attached
	electron string
	agent    bool
	ca       *proto.TrafficCA
	err      string
}

// attach attaches a target, then shows a tab made for it.
func (v *View) attach(o Options, p proto.TrafficAttachParams) {
	s := &v.inter
	if s.busy != "" || o.Host == nil {
		return
	}
	s.busy, s.err = p.Kind+p.Program+p.Tab, ""
	h := o.Host
	go func() {
		t, err := h.TrafficAttach(p)
		targets, _ := h.TrafficAttached()
		o.Update(func() {
			s.busy = ""
			if err != nil {
				s.err = client.Explain(err)
				return
			}
			v.targets = targets
			if t.Note != "" {
				v.notice = t.Note
			}
			if t.Tab != "" && o.OpenTab != nil && p.Kind == proto.TargetTerminal {
				o.OpenTab(t.Tab)
			}
		})
	}()
}

func (v *View) detach(o Options, source string) {
	h := o.Host
	go func() {
		err := h.TrafficDetach(source)
		targets, _ := h.TrafficAttached()
		o.Update(func() {
			if err != nil {
				v.inter.err = client.Explain(err)
			}
			v.targets = targets
		})
	}()
}

func (v *View) interceptPage(c *ui.Context, o Options) {
	s := &v.inter
	session := o.Session.ID
	if o.Session.Parent != "" {
		session = o.Session.Parent
	}
	ui.Scroll(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().Padding(12).Gap(12).Children(func() {
			ui.Text(c, "What runs on "+o.HostName+" can send its HTTP through enowX. Nothing is caught until you attach it.").FontSize(11.5).TextColor(theme.Muted)
			if s.err != "" {
				ui.Text(c, s.err).FontSize(12).TextColor(theme.Error)
			}
			if len(v.targets) > 0 {
				section(c, "Attached", func() {
					for _, t := range v.targets {
						ui.Row(c.Key(t.Source)).FillWidth().Gap(8).Children(func() {
							widget.Dot(c, theme.Done, false, 7)
							ui.Column(c).Grow(1).MinWidth(0).Children(func() {
								ui.Text(c, t.Label+" · "+t.Kind).FontSize(12).SingleLine()
								ui.Text(c, t.Proxy).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Muted).SingleLine()
							})
							if widget.GhostButton(c, "Detach").Clicked() {
								v.detach(o, t.Source)
							}
						})
					}
				})
			}
			if session == "" {
				ui.Text(c, "Select a session to attach to it.").FontSize(12).TextColor(theme.Muted)
				return
			}
			section(c, "Terminals", func() {
				targetRow(c, icon.Terminal, "New terminal", "A tab of this session: what you run there is caught.", "Open", s.busy != "", func() {
					v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetTerminal, Session: session})
				})
				attached := v.attachedTo(session, proto.TargetSession)
				title, body := "This session's new tabs", "Tabs opened from now on, and what the Run menu starts."
				if attached != "" {
					if widget.Button(c, widget.Secondary, icon.Stop, "Stop catching the session's new tabs", "").Clicked() {
						v.detach(o, attached)
					}
				} else {
					targetRow(c, icon.Split, title, body, "Attach", s.busy != "", func() {
						v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetSession, Session: session, Agent: s.agent})
					})
					widget.Checkbox(c, &s.agent, "The agent's own traffic too, once it starts again (its API keys show, masked)")
				}
				for _, tab := range o.Tabs {
					if tab.Exited || (tab.Agent != "shell" && tab.Agent != "") {
						continue
					}
					name := tab.Name
					if name == "" {
						name = "Terminal"
					}
					if v.attachedTo(tab.ID, proto.TargetShell) != "" || v.attachedTab(tab.ID) {
						continue
					}
					targetRow(c.Key("tab:"+tab.ID), icon.Terminal, "“"+name+"”", "This open tab: what it runs next is caught.", "Attach", s.busy != "", func() {
						v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetShell, Tab: tab.ID})
					})
				}
			})
			section(c, "Browsers", func() {
				if len(v.status.Browsers) == 0 {
					ui.Text(c, "No Chrome, Chromium, Edge, Brave or Firefox found on "+o.HostName+".").FontSize(11.5).TextColor(theme.Muted)
				}
				where := "a window of its own"
				if v.status.Display {
					where = "the session's display, shown here"
				}
				for _, b := range v.status.Browsers {
					targetRow(c.Key("b:"+b.ID), icon.Browser, b.Name, "A fresh profile, in "+where+". Your own profile isn't touched.", "Start", s.busy != "", func() {
						v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetBrowser, Session: session, Program: b.ID})
					})
				}
				ui.Row(c).FillWidth().Gap(8).Children(func() {
					ui.Icon(c, icon.AppWindow).FontSize(14).TextColor(theme.TextSoft)
					widget.Input(c, &s.electron, "An Electron app's executable on "+o.HostName).Grow(1)
					if widget.Button(c, widget.Secondary, icon.Play, "Start", "").Disabled(strings.TrimSpace(s.electron) == "" || s.busy != "").Clicked() {
						v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetElectron, Session: session, Program: strings.TrimSpace(s.electron)})
					}
				})
			})
			section(c, "Devices and containers", func() {
				targetRow(c, icon.Android, "Android", "From the Android tool: a device's traffic, with enowX's CA on emulators.", "", true, nil)
				targetRow(c, icon.Server, "Docker containers", "Containers this session starts afterwards get the proxy and the CA.", "Attach", s.busy != "", func() {
					v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetDocker, Session: session})
				})
			})
			section(c, "Anything else", func() {
				targetRow(c, icon.Globe, "An address of its own", "For a program set up by hand: an HTTP or SOCKS5 proxy on "+o.HostName+", and the CA to trust.", "Make one", s.busy != "", func() {
					v.attach(o, proto.TrafficAttachParams{Kind: proto.TargetManual, Session: session, Label: "Manual"})
				})
				if s.ca == nil {
					if widget.GhostButton(c, "Show the CA").Clicked() {
						h := o.Host
						go func() {
							ca, err := h.TrafficCA()
							o.Update(func() {
								if err != nil {
									s.err = client.Explain(err)
									return
								}
								s.ca = &ca
							})
						}()
					}
				} else {
					ui.Text(c, "SHA-256 "+s.ca.Fingerprint).Font(theme.MonoFont).FontSize(10.5).TextColor(theme.Muted).Selectable()
					ui.Text(c, "On "+o.HostName+": "+s.ca.File+" (with the system's roots: "+s.ca.Bundle+")").FontSize(11).TextColor(theme.Muted).Selectable()
					if o.CopyText != nil && widget.Button(c, widget.Secondary, icon.Copy, "Copy the CA (PEM)", "").Clicked() {
						o.CopyText(s.ca.PEM)
						v.notice = "The CA's certificate is on the clipboard."
					}
				}
			})
		})
	})
}

func (v *View) attachedTo(id, kind string) string {
	for _, t := range v.targets {
		if t.Kind == kind && (t.Session == id || t.Tab == id) {
			return t.Source
		}
	}
	return ""
}

func (v *View) attachedTab(tab string) bool {
	for _, t := range v.targets {
		if t.Tab == tab {
			return true
		}
	}
	return false
}

func section(c *ui.Context, title string, body func()) {
	ui.Column(c.Key(title)).FillWidth().Gap(8).Children(func() {
		ui.Text(c, title).FontSize(11).TextColor(theme.Muted).FontWeight(600)
		body()
	})
}

// targetRow is a target to attach: what it is, and its button (none when
// action is nil).
func targetRow(c *ui.Context, svg *ui.SVG, title, body, button string, busy bool, action func()) {
	ui.Row(c.Key(title)).FillWidth().Gap(10).Children(func() {
		ui.Icon(c, svg).FontSize(15).TextColor(theme.TextSoft).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(12.5)
			ui.Text(c, body).FontSize(11).TextColor(theme.Muted)
		})
		if action != nil && widget.Button(c, widget.Secondary, nil, button, "").Disabled(busy).Clicked() {
			action()
		}
	})
}

// pausedEdit is a paused exchange being changed before it goes on.
type pausedEdit struct {
	status  string
	headers string
	body    string
}

// pausedList shows the exchanges waiting at breakpoints, each to resume
// as it is, changed, or to drop.
func (v *View) pausedList(c *ui.Context, o Options) {
	ui.Column(c).FillWidth().Shrink(0).MaxHeight(320).Padding(8, 12).Gap(8).Background(theme.Sunken).
		BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
		ui.Scroll(c).FillWidth().MaxHeight(300).Children(func() {
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				for _, p := range v.paused {
					v.pausedRow(c.Key(fmt.Sprint(p.ID)), o, p)
				}
			})
		})
	})
}

func (v *View) pausedRow(c *ui.Context, o Options, p proto.TrafficPaused) {
	e := v.edits[p.ID]
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			widget.Tag(c, "paused "+p.Stage, theme.Warning)
			ui.Text(c, p.Method+" "+p.URL).FontSize(12).SingleLine().Grow(1).Shrink(1).MinWidth(0).Tooltip(p.URL)
			if e == nil && widget.IconButton(c, icon.Pencil, "Change it first", false).Clicked() {
				e = &pausedEdit{headers: headerLines(p.Headers), body: string(p.Body)}
				if p.Status != 0 {
					e.status = fmt.Sprint(p.Status)
				}
				v.edits[p.ID] = e
			}
			if widget.GhostButton(c, "Drop").Clicked() {
				v.resume(o, proto.TrafficResumeParams{ID: p.ID, Drop: true})
			}
			if widget.PrimaryButton(c, icon.Play, "Resume", "").Clicked() {
				params := proto.TrafficResumeParams{ID: p.ID}
				if e != nil {
					edit := &proto.TrafficEdit{Headers: parseHeaderLines(e.headers)}
					if n, err := parseNum(e.status, "Status"); err == nil {
						edit.Status = n
					}
					body := []byte(e.body)
					edit.Body = &body
					params.Edit = edit
				}
				v.resume(o, params)
			}
		})
		if e != nil {
			if p.Stage == "response" {
				widget.Input(c, &e.status, "Status").Width(90)
			}
			ui.TextAreaBase(c, &e.headers).FillWidth().MinHeight(40).MaxHeight(120).Padding(6, 8).Font(theme.MonoFont).FontSize(11).
				Radius(theme.RadiusControl).Background(theme.Background).Border(1, theme.Border)
			ui.TextAreaBase(c, &e.body).FillWidth().MinHeight(60).MaxHeight(160).Padding(6, 8).Font(theme.MonoFont).FontSize(11).
				Radius(theme.RadiusControl).Background(theme.Background).Border(1, theme.Border)
		}
	})
}

func (v *View) resume(o Options, p proto.TrafficResumeParams) {
	delete(v.edits, p.ID)
	h := o.Host
	go func() {
		err := h.TrafficResume(p)
		o.Update(func() {
			if err != nil {
				v.notice = client.Explain(err)
			}
		})
	}()
}
