package ui

import (
	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/hostconn"
	"github.com/enowx/enowx/proto"
)

// trustPrompt asks the user whether a server's host key is the one they
// expect; the connecting goroutine waits on answer.
type trustPrompt struct {
	addr        string
	fingerprint string
	answer      chan bool
}

// connect drops the current connection and connects to h in the
// background. Sessions keep running in the daemon left behind.
func (a *App) connect(h hosts.Host) {
	a.disconnect()
	a.gen++
	gen := a.gen
	a.host = h
	a.connecting, a.connErr, a.status = "Connecting to "+h.Name+"…", "", ""
	if h.ID == hosts.LocalID {
		a.connecting = "Starting…"
	}
	opts := hostconn.Options{
		OnEvent:       func(ev proto.Event) { a.handleEvent(gen, ev) },
		DaemonCommand: a.opts.DaemonCommand,
		Progress: func(s string) {
			a.win.Update(func() {
				if gen == a.gen && a.connecting != "" {
					a.connecting = s + "…"
				}
			})
		},
		TrustHost: func(addr, fingerprint string) bool { return a.askTrust(gen, addr, fingerprint) },
		Password:  a.password,
	}
	openProject := ""
	if gen == 1 {
		openProject = a.opts.OpenProject
	}
	go func() {
		c, err := hostconn.Connect(h, opts)
		a.win.Update(func() {
			if gen != a.gen {
				if c != nil {
					c.Close()
				}
				return
			}
			a.connecting = ""
			if err != nil {
				a.connErr = err.Error()
				return
			}
			a.client = c
			go a.refresh(c, gen, openProject)
			go a.watchDisconnect(c, gen)
		})
	}()
}

func (a *App) watchDisconnect(c *client.Client, gen int) {
	<-c.Done()
	a.win.Update(func() {
		if gen == a.gen && a.client == c {
			a.client = nil
			a.connErr = "Disconnected from " + a.host.Name + ". Its sessions keep running."
		}
	})
}

// disconnect closes the connection and forgets its state.
func (a *App) disconnect() {
	for _, p := range a.panes {
		p.close()
	}
	if a.client != nil {
		a.client.Close()
		a.client = nil
	}
	if a.trust != nil {
		a.trust.answer <- false
		a.trust = nil
	}
	a.agents, a.projects, a.sessions = nil, nil, nil
	a.panes, a.removed = map[string]*pane{}, map[string]bool{}
	a.project, a.selected, a.maximized = "", "", ""
	a.newSession.open, a.addProject.open = false, false
}

// askTrust runs on the connecting goroutine and waits for the user.
func (a *App) askTrust(gen int, addr, fingerprint string) bool {
	answer := make(chan bool, 1)
	a.win.Update(func() {
		if gen != a.gen {
			answer <- false
			return
		}
		a.trust = &trustPrompt{addr: addr, fingerprint: fingerprint, answer: answer}
	})
	return <-answer
}

func (a *App) answerTrust(yes bool) {
	if a.trust != nil {
		a.trust.answer <- yes
		a.trust = nil
	}
}

func (a *App) reloadHosts() {
	if a.opts.Hosts != nil {
		a.hostList = a.opts.Hosts.List()
	}
}

// allHosts is this machine followed by the stored servers, plus the
// current host when it came from the command line and isn't stored.
func (a *App) allHosts() []hosts.Host {
	out := append([]hosts.Host{LocalHost()}, a.hostList...)
	for _, h := range out {
		if h.ID == a.host.ID {
			return out
		}
	}
	if a.host.ID != "" {
		out = append(out, a.host)
	}
	return out
}
