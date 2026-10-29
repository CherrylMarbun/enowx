package ui

import (
	"errors"
	"strings"
	"sync"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// An agent reading other sessions, asked by the user: the app reaches the
// hosts it is connected to for it (daemon/sessionpeek.go).

// sessionsEverywhere lists the sessions of every host the app reaches but
// from, the asking host's daemon.
func (a *App) sessionsEverywhere(from string, reply func(any, error)) {
	var hosts []hostConn
	for _, h := range a.reachable() {
		if h.info.ID != from {
			hosts = append(hosts, h)
		}
	}
	go func() {
		out := make([]proto.HostSessions, len(hosts))
		var wg sync.WaitGroup
		for i, h := range hosts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				hs := proto.HostSessions{Host: h.info.ID, HostName: h.host.Name, Sessions: []proto.SessionInfo{}}
				list, err := h.cl.Sessions()
				if err != nil {
					hs.Err = client.Explain(err)
				} else {
					hs.Sessions = list
				}
				out[i] = hs
			}()
		}
		wg.Wait()
		reply(out, nil)
	}()
}

// peekSession reads a session's conversation on the host p names.
func (a *App) peekSession(p proto.SessionPeekParams, reply func(any, error)) {
	var target hostConn
	found := false
	for _, h := range a.reachable() {
		if h.info.ID == p.Host || h.host.ID == p.Host || strings.EqualFold(h.host.Name, p.Host) {
			target, found = h, true
			break
		}
	}
	if !found {
		var names []string
		for _, h := range a.reachable() {
			names = append(names, h.host.Name)
		}
		reply(nil, errors.New("no host “"+p.Host+"” in the app's reach now: "+strings.Join(names, ", ")))
		return
	}
	p.Host = ""
	go func() {
		out, err := target.cl.PeekSession(p)
		if err == nil {
			out.HostName = target.host.Name
		}
		reply(out, err)
	}()
}
